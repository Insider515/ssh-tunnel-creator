package tunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testUser и testPass — учётные данные встроенного jump-сервера в тестах.
const (
	testUser = "tester"
	testPass = "secret"
)

// newTestSigner создаёт хост-ключ для встроенного SSH-сервера.
func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("не удалось сгенерировать ключ: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("не удалось создать подписывающего: %v", err)
	}
	return signer
}

// startEchoServer поднимает TCP-сервер, который возвращает обратно всё полученное.
// Играет роль целевого сервера.
func startEchoServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось поднять echo-сервер: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// startJumpServer поднимает минимальный SSH-сервер, который принимает пароль
// и выполняет запросы direct-tcpip — ровно то, на чём держится туннель.
func startJumpServer(t *testing.T) (host string, port int) {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == testUser && string(pass) == testPass {
				return nil, nil
			}
			return nil, errors.New("неверный пароль")
		},
	}
	cfg.AddHostKey(newTestSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось поднять jump-сервер: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveJump(c, cfg)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// serveJump обслуживает одно соединение встроенного jump-сервера:
// аутентифицирует клиента и открывает запрошенные каналы direct-tcpip.
func serveJump(c net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for nc := range chans {
		if nc.ChannelType() != "direct-tcpip" {
			_ = nc.Reject(ssh.UnknownChannelType, "поддерживается только direct-tcpip")
			continue
		}
		var req struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if err := ssh.Unmarshal(nc.ExtraData(), &req); err != nil {
			_ = nc.Reject(ssh.ConnectionFailed, "некорректный запрос")
			continue
		}
		target := net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port)))
		remote, err := net.DialTimeout("tcp", target, 3*time.Second)
		if err != nil {
			_ = nc.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			remote.Close()
			continue
		}
		go ssh.DiscardRequests(chReqs)
		go func() {
			defer remote.Close()
			defer ch.Close()
			done := make(chan struct{})
			go func() { _, _ = io.Copy(ch, remote); close(done) }()
			_, _ = io.Copy(remote, ch)
			<-done
		}()
	}
}

// freePort находит свободный локальный TCP-порт.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось найти свободный порт: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// testConfig собирает параметры туннеля для тестов.
func testConfig(localPort int, jumpHost string, jumpPort int, targetHost string, targetPort int) Config {
	return Config{
		LocalPort:  localPort,
		JumpHost:   jumpHost,
		JumpPort:   jumpPort,
		JumpUser:   testUser,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Auth: AuthConfig{
			Method:   AuthPassword,
			Password: testPass,
		},
		VerifyHostKey:  false,
		ConnectTimeout: 5 * time.Second,
		KeepAlive:      time.Second,
	}
}

// TestTunnelForwardsData проверяет сквозную работу: данные, записанные в локальный порт,
// доходят до целевого сервера через jump-сервер и возвращаются обратно.
func TestTunnelForwardsData(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("туннель не запустился: %v", err)
	}
	defer tun.Stop()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("не удалось подключиться к локальному порту туннеля: %v", err)
	}
	defer conn.Close()

	want := "проверка связи"
	if _, err := conn.Write([]byte(want)); err != nil {
		t.Fatalf("запись в туннель не удалась: %v", err)
	}
	buf := make([]byte, len(want))
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("чтение из туннеля не удалось: %v", err)
	}
	if string(buf) != want {
		t.Fatalf("через туннель пришло %q, ожидалось %q", buf, want)
	}

	if got := tun.Stats().Total; got != 1 {
		t.Fatalf("счётчик соединений равен %d, ожидался 1", got)
	}
}

// TestStartDoesNotOpenPortWhenTargetUnreachable проверяет главное исправление
// относительно Electron-версии: если целевой узел недоступен, запуск завершается ошибкой
// и локальный порт не занимается, а не сообщает об "активном туннеле".
func TestStartDoesNotOpenPortWhenTargetUnreachable(t *testing.T) {
	jumpHost, jumpPort := startJumpServer(t)
	deadPort := freePort(t) // порт, на котором никто не слушает
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, "127.0.0.1", deadPort), Callbacks{})
	err := tun.Start(context.Background())
	if err == nil {
		tun.Stop()
		t.Fatal("запуск завершился успехом, хотя целевой узел недоступен")
	}
	if !strings.Contains(err.Error(), "не смог открыть соединение") {
		t.Fatalf("ошибка не указывает на недоступность целевого узла: %v", err)
	}
	if tun.Running() {
		t.Fatal("туннель считается запущенным после неудачной проверки")
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)))
	if err != nil {
		t.Fatalf("локальный порт %d остался занят после неудачного запуска: %v", localPort, err)
	}
	ln.Close()
}

// TestCheckDetectsWrongPassword проверяет, что ошибка аутентификации выявляется
// на этапе проверки, а не при первом подключении клиента.
func TestCheckDetectsWrongPassword(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)

	cfg := testConfig(freePort(t), jumpHost, jumpPort, echoHost, echoPort)
	cfg.Auth.Password = "неверный"
	tun := New(cfg, Callbacks{})

	err := tun.Check(context.Background())
	if err == nil {
		t.Fatal("проверка прошла с неверным паролем")
	}
	if !strings.Contains(err.Error(), "не установлено") {
		t.Fatalf("ошибка не указывает на неудачу SSH-соединения: %v", err)
	}
}

// TestCheckDetectsUnreachableJump проверяет, что недоступный jump-сервер
// распознаётся отдельно от остальных причин.
func TestCheckDetectsUnreachableJump(t *testing.T) {
	cfg := testConfig(freePort(t), "127.0.0.1", freePort(t), "127.0.0.1", 22)
	tun := New(cfg, Callbacks{})

	err := tun.Check(context.Background())
	if err == nil {
		t.Fatal("проверка прошла при недоступном jump-сервере")
	}
	if !strings.Contains(err.Error(), "недостижим по TCP") {
		t.Fatalf("ошибка не указывает на недоступность jump-сервера: %v", err)
	}
}

// TestCheckDetectsBusyLocalPort проверяет, что занятый локальный порт
// сообщается явно, а не приводит к молчаливому отказу.
func TestCheckDetectsBusyLocalPort(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось занять порт: %v", err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	tun := New(testConfig(busyPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	err = tun.Check(context.Background())
	if err == nil {
		t.Fatal("проверка прошла при занятом локальном порте")
	}
	if !strings.Contains(err.Error(), "занят") {
		t.Fatalf("ошибка не указывает на занятый порт: %v", err)
	}
}

// TestValidateRejectsBadFields проверяет, что незаполненные и нечисловые параметры
// отвергаются до обращения к сети.
func TestValidateRejectsBadFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"нулевой локальный порт", Config{LocalPort: 0}, "локальный порт"},
		{"нет адреса jump", Config{LocalPort: 8080}, "jump-сервера"},
		{"нет пользователя", Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22}, "имя пользователя"},
		{"нет целевого адреса", Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22, JumpUser: "u"}, "целевого сервера"},
		{
			"неизвестный способ аутентификации",
			Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22, JumpUser: "u", TargetHost: "t", TargetPort: 22},
			"способ аутентификации",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if err == nil {
				t.Fatal("ошибка не возвращена")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("ошибка %q не содержит %q", err, c.want)
			}
		})
	}
}

// TestStopClosesActiveConnections проверяет, что остановка разрывает уже открытые
// соединения: закрытие слушателя само их не рвёт.
func TestStopClosesActiveConnections(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("туннель не запустился: %v", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("не удалось подключиться к туннелю: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatalf("запись не удалась: %v", err)
	}
	buf := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("эхо не получено: %v", err)
	}

	tun.Stop()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("соединение осталось живым после остановки туннеля")
	}
	if got := tun.Stats().Active; got != 0 {
		t.Fatalf("после остановки активных соединений %d, ожидалось 0", got)
	}
}

// TestKeyAuthReportsMissingFile проверяет, что отсутствующий файл ключа
// объясняется явно, а не превращается в общую ошибку рукопожатия.
func TestKeyAuthReportsMissingFile(t *testing.T) {
	_, err := buildAuth(AuthConfig{Method: AuthKey, KeyPath: "/нет/такого/файла"})
	if err == nil {
		t.Fatal("ошибка не возвращена")
	}
	if !strings.Contains(err.Error(), "не удалось прочитать файл ключа") {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

// TestPasswordAuthRequiresPassword проверяет, что пустой пароль отсекается до соединения.
func TestPasswordAuthRequiresPassword(t *testing.T) {
	_, err := buildAuth(AuthConfig{Method: AuthPassword})
	if err == nil || !strings.Contains(err.Error(), "пароль не задан") {
		t.Fatalf("неожиданный результат: %v", err)
	}
}

// TestHostKeyMismatchIsRejected проверяет, что несовпадение ключа хоста
// с сохранённым ранее приводит к отказу, а не к молчаливому доверию.
func TestHostKeyMismatchIsRejected(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/known_hosts"

	stored := newTestSigner(t)
	if err := ensureFile(file); err != nil {
		t.Fatalf("не удалось создать файл: %v", err)
	}
	if err := appendKnownHost(file, "127.0.0.1:2222", stored.PublicKey()); err != nil {
		t.Fatalf("не удалось записать ключ: %v", err)
	}

	cb, err := newHostKeyCallback(true, file, nil, func(string, ...any) {})
	if err != nil {
		t.Fatalf("не удалось построить проверку: %v", err)
	}

	other := newTestSigner(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
	err = cb("127.0.0.1:2222", addr, other.PublicKey())
	if err == nil {
		t.Fatal("подменённый ключ хоста принят")
	}
	if !strings.Contains(err.Error(), "не совпадает") {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if err := cb("127.0.0.1:2222", addr, stored.PublicKey()); err != nil {
		t.Fatalf("сохранённый ключ отвергнут: %v", err)
	}
}

// TestHostKeyUnknownRequiresConfirmation проверяет, что незнакомый ключ хоста
// без подтверждения оператора не принимается, а после подтверждения запоминается.
func TestHostKeyUnknownRequiresConfirmation(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/known_hosts"
	signer := newTestSigner(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}

	deny, err := newHostKeyCallback(true, file, func(string, string, string) bool { return false }, func(string, ...any) {})
	if err != nil {
		t.Fatalf("не удалось построить проверку: %v", err)
	}
	if err := deny("127.0.0.1:2222", addr, signer.PublicKey()); err == nil {
		t.Fatal("неподтверждённый ключ принят")
	}

	allow, err := newHostKeyCallback(true, file, func(string, string, string) bool { return true }, func(string, ...any) {})
	if err != nil {
		t.Fatalf("не удалось построить проверку: %v", err)
	}
	if err := allow("127.0.0.1:2222", addr, signer.PublicKey()); err != nil {
		t.Fatalf("подтверждённый ключ отвергнут: %v", err)
	}

	again, err := newHostKeyCallback(true, file, func(string, string, string) bool {
		t.Fatal("ключ не был запомнён: оператора спросили повторно")
		return false
	}, func(string, ...any) {})
	if err != nil {
		t.Fatalf("не удалось построить проверку: %v", err)
	}
	if err := again("127.0.0.1:2222", addr, signer.PublicKey()); err != nil {
		t.Fatalf("запомненный ключ отвергнут: %v", err)
	}
}

// TestReconnectAfterJumpLoss проверяет, что обрыв связи с jump-сервером обнаруживается
// и при включённом автопереподключении туннель восстанавливается без перезапуска вручную.
func TestReconnectAfterJumpLoss(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == testUser && string(pass) == testPass {
				return nil, nil
			}
			return nil, errors.New("неверный пароль")
		},
	}
	cfg.AddHostKey(newTestSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось поднять jump-сервер: %v", err)
	}
	defer ln.Close()
	jumpAddr := ln.Addr().(*net.TCPAddr)

	// Сохраняем ссылки на принятые соединения, чтобы оборвать их принудительно.
	accepted := make(chan net.Conn, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c
			go serveJump(c, cfg)
		}
	}()

	localPort := freePort(t)
	tcfg := testConfig(localPort, jumpAddr.IP.String(), jumpAddr.Port, echoHost, echoPort)
	tcfg.AutoReconnect = true
	tcfg.KeepAlive = 300 * time.Millisecond

	statuses := make(chan Status, 32)
	tun := New(tcfg, Callbacks{OnStatus: func(s Status, _ string) {
		select {
		case statuses <- s:
		default:
		}
	}})

	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("туннель не запустился: %v", err)
	}
	defer tun.Stop()

	// Рвём соединение, которое туннель установил с jump-сервером.
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("jump-сервер не получил соединения от туннеля")
	}

	deadline := time.After(15 * time.Second)
	sawReconnecting := false
	for {
		select {
		case s := <-statuses:
			if s == StatusReconnecting {
				sawReconnecting = true
			}
			if s == StatusActive && sawReconnecting {
				goto restored
			}
		case <-deadline:
			t.Fatalf("туннель не восстановился: переподключение замечено=%v", sawReconnecting)
		}
	}

restored:
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("после восстановления подключение не прошло: %v", err)
	}
	defer conn.Close()
	msg := []byte("после обрыва")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("запись не удалась: %v", err)
	}
	buf := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("эхо после восстановления не получено: %v", err)
	}
	if string(buf) != string(msg) {
		t.Fatalf("получено %q, ожидалось %q", buf, msg)
	}
}
