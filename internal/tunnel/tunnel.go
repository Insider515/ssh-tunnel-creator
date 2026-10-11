// Package tunnel реализует TCP-туннель через промежуточный (jump) SSH-сервер:
// локальный порт на машине оператора прокидывается на target_host:target_port,
// куда jump-сервер открывает соединение от своего имени.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// Status — состояние туннеля для отображения в интерфейсе.
type Status string

const (
	// StatusIdle — туннель не запущен.
	StatusIdle Status = "неактивен"
	// StatusChecking — идёт проверка или установка соединения.
	StatusChecking Status = "проверка"
	// StatusActive — туннель работает, локальный порт слушается.
	StatusActive Status = "активен"
	// StatusReconnecting — связь с jump-сервером потеряна, идут попытки восстановления.
	StatusReconnecting Status = "переподключение"
	// StatusError — последняя операция завершилась ошибкой.
	StatusError Status = "ошибка"
)

// Config — полный набор параметров туннеля.
type Config struct {
	LocalHost string // адрес локального слушателя; пусто означает 127.0.0.1
	LocalPort int

	JumpHost string
	JumpPort int
	JumpUser string
	Auth     AuthConfig

	TargetHost string
	TargetPort int

	VerifyHostKey  bool
	KnownHostsFile string
	HostKeyPrompt  HostKeyPrompt

	ConnectTimeout time.Duration
	KeepAlive      time.Duration
	AutoReconnect  bool
}

// Stats — счётчики соединений для интерфейса.
type Stats struct {
	Active int    // открытых прямо сейчас
	Total  uint64 // обслужено с момента запуска
}

// Callbacks — функции, через которые туннель сообщает интерфейсу о своём состоянии.
// Вызываются из рабочих горутин, поэтому реализация обязана сама переносить
// обновления в поток интерфейса.
type Callbacks struct {
	OnStatus func(Status, string)
	OnLog    func(string)
	OnStats  func(Stats)
}

// Tunnel — жизненный цикл одного туннеля: проверка, запуск, перекачка данных, остановка.
type Tunnel struct {
	cfg Config
	cb  Callbacks

	mu       sync.Mutex
	client   *ssh.Client
	listener net.Listener
	conns    map[io.Closer]struct{}
	cancel   context.CancelFunc
	running  bool

	total atomic.Uint64
}

// New создаёт туннель с заданными параметрами и подставляет значения по умолчанию
// для таймаута соединения, keepalive и локального адреса.
func New(cfg Config, cb Callbacks) *Tunnel {
	if cfg.LocalHost == "" {
		cfg.LocalHost = "127.0.0.1"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 15 * time.Second
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 15 * time.Second
	}
	return &Tunnel{cfg: cfg, cb: cb}
}

// Validate проверяет параметры до попытки соединения, чтобы пустое или нечисловое
// поле в интерфейсе не превращалось в случайно выбранный порт или невнятную ошибку сети.
func (c Config) Validate() error {
	if err := validatePort("локальный порт", c.LocalPort); err != nil {
		return err
	}
	if c.JumpHost == "" {
		return errors.New("не указан адрес jump-сервера")
	}
	if err := validatePort("порт jump-сервера", c.JumpPort); err != nil {
		return err
	}
	if c.JumpUser == "" {
		return errors.New("не указано имя пользователя jump-сервера")
	}
	if c.TargetHost == "" {
		return errors.New("не указан адрес целевого сервера")
	}
	if err := validatePort("порт целевого сервера", c.TargetPort); err != nil {
		return err
	}
	switch c.Auth.Method {
	case AuthPassword, AuthKey, AuthAgent:
	default:
		return fmt.Errorf("неизвестный способ аутентификации: %q", c.Auth.Method)
	}
	return nil
}

// validatePort проверяет, что номер порта попадает в допустимый диапазон TCP.
func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s должен быть числом от 1 до 65535, получено %d", name, port)
	}
	return nil
}

// LocalAddr — адрес, который слушает туннель на машине оператора.
func (c Config) LocalAddr() string {
	return net.JoinHostPort(c.LocalHost, strconv.Itoa(c.LocalPort))
}

// JumpAddr — адрес промежуточного SSH-сервера.
func (c Config) JumpAddr() string {
	return net.JoinHostPort(c.JumpHost, strconv.Itoa(c.JumpPort))
}

// TargetAddr — адрес конечного сервера, куда jump-сервер открывает соединение.
func (c Config) TargetAddr() string {
	return net.JoinHostPort(c.TargetHost, strconv.Itoa(c.TargetPort))
}

// Running сообщает, запущен ли туннель.
func (t *Tunnel) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// Check выполняет настоящую проверку соединения и ничего не оставляет запущенным:
// TCP до jump-сервера, SSH-рукопожатие с проверкой ключа хоста, аутентификация,
// открытие канала до целевого узла и проверка, что локальный порт свободен.
// Успех означает, что все четыре условия выполнены на самом деле.
func (t *Tunnel) Check(ctx context.Context) error {
	if err := t.cfg.Validate(); err != nil {
		t.status(StatusError, err.Error())
		return err
	}
	t.status(StatusChecking, "проверка соединения")

	client, err := t.connect(ctx)
	if err != nil {
		t.status(StatusError, err.Error())
		return err
	}
	client.Close()

	if !t.Running() {
		ln, err := net.Listen("tcp", t.cfg.LocalAddr())
		if err != nil {
			e := fmt.Errorf("локальный адрес %s занят другим процессом: %w", t.cfg.LocalAddr(), err)
			t.status(StatusError, e.Error())
			return e
		}
		ln.Close()
		t.logf("локальный адрес %s свободен", t.cfg.LocalAddr())
	}

	msg := fmt.Sprintf("проверка пройдена: %s доступен через %s", t.cfg.TargetAddr(), t.cfg.JumpAddr())
	t.status(StatusIdle, msg)
	return nil
}

// Start проверяет соединение и, только если проверка прошла, занимает локальный порт
// и начинает обслуживать клиентов. Обратное к Electron-версии поведение: там статус
// "активен" выставлялся по факту открытия локального порта, то есть ещё до того,
// как выяснялось, доступен ли целевой узел.
func (t *Tunnel) Start(parent context.Context) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return errors.New("туннель уже запущен")
	}
	t.mu.Unlock()

	if err := t.cfg.Validate(); err != nil {
		t.status(StatusError, err.Error())
		return err
	}

	ctx, cancel := context.WithCancel(parent)

	t.status(StatusChecking, "установка соединения")
	client, err := t.connect(ctx)
	if err != nil {
		cancel()
		t.status(StatusError, err.Error())
		return err
	}

	ln, err := net.Listen("tcp", t.cfg.LocalAddr())
	if err != nil {
		client.Close()
		cancel()
		e := fmt.Errorf("не удалось занять локальный адрес %s: %w", t.cfg.LocalAddr(), err)
		t.status(StatusError, e.Error())
		return e
	}

	t.mu.Lock()
	t.client = client
	t.listener = ln
	t.cancel = cancel
	t.conns = make(map[io.Closer]struct{})
	t.running = true
	t.mu.Unlock()

	t.total.Store(0)
	t.emitStats()

	go t.acceptLoop(ctx, ln)
	go t.monitor(ctx)

	t.status(StatusActive, fmt.Sprintf("туннель активен, подключайте клиентов к %s", t.cfg.LocalAddr()))
	return nil
}

// Stop останавливает туннель: закрывает слушателя, принудительно разрывает уже
// открытые соединения (закрытие слушателя их не рвёт) и закрывает SSH-соединение.
func (t *Tunnel) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	t.running = false
	cancel, ln, client := t.cancel, t.listener, t.client
	open := make([]io.Closer, 0, len(t.conns))
	for c := range t.conns {
		open = append(open, c)
	}
	t.cancel, t.listener, t.client, t.conns = nil, nil, nil, nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range open {
		_ = c.Close()
	}
	if client != nil {
		_ = client.Close()
	}

	t.emitStats()
	t.status(StatusIdle, "туннель остановлен")
}

// connect устанавливает соединение с jump-сервером и проверяет, что через него
// действительно открывается канал до целевого узла. Каждая стадия отчитывается в журнал
// отдельно, поэтому по сообщению видно, что именно не сработало.
func (t *Tunnel) connect(ctx context.Context) (*ssh.Client, error) {
	auth, err := buildAuth(t.cfg.Auth)
	if err != nil {
		return nil, err
	}
	hostKey, err := newHostKeyCallback(t.cfg.VerifyHostKey, t.cfg.KnownHostsFile, t.cfg.HostKeyPrompt, t.logf)
	if err != nil {
		return nil, err
	}

	jumpAddr := t.cfg.JumpAddr()
	t.logf("1/3 TCP-соединение с jump-сервером %s", jumpAddr)
	dialer := net.Dialer{Timeout: t.cfg.ConnectTimeout}
	raw, err := dialer.DialContext(ctx, "tcp", jumpAddr)
	if err != nil {
		return nil, fmt.Errorf("jump-сервер %s недостижим по TCP: %w", jumpAddr, err)
	}

	t.logf("2/3 SSH-рукопожатие и аутентификация пользователя %s способом %q", t.cfg.JumpUser, t.cfg.Auth.Method)
	_ = raw.SetDeadline(time.Now().Add(t.cfg.ConnectTimeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(raw, jumpAddr, &ssh.ClientConfig{
		User:            t.cfg.JumpUser,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         t.cfg.ConnectTimeout,
	})
	if err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("SSH-соединение с jump-сервером %s не установлено: %w", jumpAddr, err)
	}
	_ = raw.SetDeadline(time.Time{})
	client := ssh.NewClient(sshConn, chans, reqs)

	targetAddr := t.cfg.TargetAddr()
	t.logf("3/3 проверка канала до целевого узла %s через jump-сервер", targetAddr)
	probeCtx, cancel := context.WithTimeout(ctx, t.cfg.ConnectTimeout)
	defer cancel()
	probe, err := client.DialContext(probeCtx, "tcp", targetAddr)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("jump-сервер не смог открыть соединение с %s: %w", targetAddr, err)
	}
	_ = probe.Close()

	t.logf("соединение установлено: %s доступен через %s", targetAddr, jumpAddr)
	return client, nil
}

// acceptLoop принимает локальные подключения и отдаёт каждое в отдельную горутину.
func (t *Tunnel) acceptLoop(ctx context.Context, ln net.Listener) {
	for {
		local, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			t.logf("ошибка приёма локального подключения: %v", err)
			continue
		}
		go t.handle(ctx, local)
	}
}

// handle обслуживает одно локальное подключение: открывает канал до целевого узла
// через jump-сервер и перекачивает данные в обе стороны. Любая ошибка остаётся
// внутри этой горутины и попадает в журнал, процесс от неё не падает.
func (t *Tunnel) handle(ctx context.Context, local net.Conn) {
	id := t.total.Add(1)
	t.track(local)
	t.emitStats()
	defer func() {
		t.untrack(local)
		_ = local.Close()
		t.emitStats()
	}()

	client := t.currentClient()
	if client == nil {
		t.logf("подключение #%d отклонено: нет связи с jump-сервером", id)
		return
	}

	dialCtx, cancel := context.WithTimeout(ctx, t.cfg.ConnectTimeout)
	defer cancel()
	remote, err := client.DialContext(dialCtx, "tcp", t.cfg.TargetAddr())
	if err != nil {
		t.logf("подключение #%d: jump-сервер не открыл канал до %s: %v", id, t.cfg.TargetAddr(), err)
		return
	}
	t.track(remote)
	defer func() {
		t.untrack(remote)
		_ = remote.Close()
	}()

	t.logf("подключение #%d открыто: %s -> %s", id, local.RemoteAddr(), t.cfg.TargetAddr())
	up, down := pipe(local, remote)
	t.logf("подключение #%d закрыто: отправлено %d Б, получено %d Б", id, up, down)
}

// pipe перекачивает данные между локальным подключением и каналом до целевого узла
// в обе стороны и возвращает объёмы переданных байт.
func pipe(local, remote net.Conn) (up, down int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		up, _ = io.Copy(remote, local)
		closeWrite(remote)
	}()
	go func() {
		defer wg.Done()
		down, _ = io.Copy(local, remote)
		closeWrite(local)
	}()
	wg.Wait()
	return up, down
}

// closeWrite закрывает только направление записи, если соединение это умеет,
// чтобы противоположная сторона получила EOF, а не ждала данных до таймаута.
func closeWrite(c net.Conn) {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

// monitor следит за живостью SSH-соединения: ждёт его завершения и параллельно
// посылает keepalive-запросы. Разрыв фиксируется сразу, а не обнаруживается
// случайным клиентом через неопределённое время.
func (t *Tunnel) monitor(ctx context.Context) {
	for {
		client := t.currentClient()
		if client == nil {
			return
		}

		dead := make(chan error, 1)
		go func() { dead <- client.Wait() }()

		ticker := time.NewTicker(t.cfg.KeepAlive)
		var reason error
	watch:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case err := <-dead:
				reason = err
				ticker.Stop()
				break watch
			case <-ticker.C:
				// Ответ "не поддерживается" — тоже признак живого соединения,
				// поэтому проверяется только транспортная ошибка.
				if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					reason = err
					ticker.Stop()
					break watch
				}
			}
		}

		if ctx.Err() != nil {
			return
		}

		t.logf("связь с jump-сервером потеряна: %v", reason)
		_ = client.Close()
		t.setClient(nil)

		if !t.cfg.AutoReconnect {
			t.status(StatusError, "связь с jump-сервером потеряна, туннель остановлен")
			t.Stop()
			return
		}
		if !t.reconnect(ctx) {
			return
		}
	}
}

// reconnect восстанавливает связь с jump-сервером с нарастающей паузой между попытками.
// Локальный слушатель при этом не закрывается: клиенты, подключившиеся в момент обрыва,
// получают отказ с записью в журнал, а после восстановления связи обслуживание продолжается.
func (t *Tunnel) reconnect(ctx context.Context) bool {
	delay := time.Second
	for attempt := 1; ; attempt++ {
		t.status(StatusReconnecting, fmt.Sprintf("переподключение, попытка %d", attempt))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}

		client, err := t.connect(ctx)
		if err == nil {
			t.setClient(client)
			t.status(StatusActive, fmt.Sprintf("связь восстановлена, туннель активен на %s", t.cfg.LocalAddr()))
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		t.logf("попытка %d не удалась: %v", attempt, err)

		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
}

// currentClient возвращает действующее SSH-соединение или nil, если связи нет.
func (t *Tunnel) currentClient() *ssh.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client
}

// setClient заменяет действующее SSH-соединение.
func (t *Tunnel) setClient(c *ssh.Client) {
	t.mu.Lock()
	t.client = c
	t.mu.Unlock()
}

// track запоминает открытое соединение, чтобы Stop мог его принудительно закрыть.
func (t *Tunnel) track(c io.Closer) {
	t.mu.Lock()
	if t.conns != nil {
		t.conns[c] = struct{}{}
	}
	t.mu.Unlock()
}

// untrack убирает закрытое соединение из списка отслеживаемых.
func (t *Tunnel) untrack(c io.Closer) {
	t.mu.Lock()
	if t.conns != nil {
		delete(t.conns, c)
	}
	t.mu.Unlock()
}

// Stats возвращает текущие счётчики соединений.
func (t *Tunnel) Stats() Stats {
	return Stats{Active: t.countLocal(), Total: t.total.Load()}
}

// countLocal считает открытые локальные подключения. В списке отслеживаемых лежат
// обе половины каждого соединения, поэтому считаются только локальные TCP-подключения,
// иначе одно подключение учитывалось бы дважды.
func (t *Tunnel) countLocal() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for c := range t.conns {
		if _, ok := c.(*net.TCPConn); ok {
			n++
		}
	}
	return n
}

// status сообщает интерфейсу новое состояние и дублирует сообщение в журнал.
func (t *Tunnel) status(s Status, msg string) {
	if t.cb.OnStatus != nil {
		t.cb.OnStatus(s, msg)
	}
	t.logf("%s", msg)
}

// emitStats отдаёт интерфейсу обновлённые счётчики соединений.
func (t *Tunnel) emitStats() {
	if t.cb.OnStats != nil {
		t.cb.OnStats(t.Stats())
	}
}

// logf пишет строку в журнал интерфейса.
func (t *Tunnel) logf(format string, args ...any) {
	if t.cb.OnLog != nil {
		t.cb.OnLog(fmt.Sprintf(format, args...))
	}
}
