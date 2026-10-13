package i18n

import "sync/atomic"

// Language identifies a supported user-interface language.
type Language string

const (
	English   Language = "en"
	Ukrainian Language = "uk"
)

var activeLanguage atomic.Value

var ukrainian = map[string]string{
	"Add profile":                                                 "Додати профіль",
	"Address":                                                     "Адреса",
	"Address or hostname":                                         "Адреса або ім'я вузла",
	"Agent is not responding: %w":                                 "Агент SSH не відповідає: %w",
	"An encrypted key requires a passphrase":                      "Зашифрований ключ потребує парольної фрази",
	"Application executable path is empty":                        "Шлях до виконуваного файла програми порожній",
	"Application executable path must be absolute":                "Шлях до виконуваного файла програми має бути абсолютним",
	"Authentication":                                              "Автентифікація",
	"Automatic reconnect after connection loss":                   "Автоматично відновлювати з'єднання після обриву",
	"Automatic startup is not supported on this operating system": "Автозапуск не підтримується цією операційною системою",
	"Browse":              "Огляд",
	"Cancel":              "Скасувати",
	"Check connection":    "Перевірити з'єднання",
	"Checking connection": "Перевірка з'єднання",
	"Clear":               "Очистити",
	"Close":               "Закрити",
	"Connect":             "Підключити",
	"Connect to the default profile when the application starts": "Підключатися до профілю за замовчуванням під час запуску програми",
	"Connections: active %d, total %d":                           "З'єднання: активних %d, усього %d",
	"Copy":                                                       "Копіювати",
	"Could not determine the home directory: %w":                 "Не вдалося визначити домашній каталог: %w",
	"Could not create directory %s: %w":                          "Не вдалося створити каталог %s: %w",
	"Could not open a connection to ssh-agent at %s: %w":         "Не вдалося підключитися до агента SSH за адресою %s: %w",
	"Could not read %s: %w":                                      "Не вдалося прочитати %s: %w",
	"Could not replace %s: %w":                                   "Не вдалося замінити %s: %w",
	"Could not write %s: %w":                                     "Не вдалося записати %s: %w",
	"Create profile":                                             "Створити профіль",
	"Default connection profile":                                 "Профіль з'єднання за замовчуванням",
	"Delete":                                                     "Видалити",
	"Delete profile":                                             "Видалити профіль",
	"Do you trust this key and want to remember it?":             "Довіряти цьому ключу й запам'ятати його?",
	"Edit profile":                                               "Налаштувати профіль",
	"English":                                                    "Англійська",
	"Enter a profile name":                                       "Укажіть назву профілю",
	"Export":                                                     "Експорт",
	"File %s is corrupted: %w":                                   "Файл %s пошкоджений: %w",
	"File key":                                                   "Файл ключа",
	"Host":                                                       "Вузол",
	"Host key":                                                   "Ключ вузла",
	"Host key %s (%s %s) was not trusted":                        "Ключ вузла %s (%s %s) не підтверджено",
	"Host key checking is disabled; the connection is vulnerable to server impersonation": "Перевірку ключа вузла вимкнено; з'єднання вразливе до підміни сервера",
	"Host key for %s differs from the previously saved key (received %s %s). The server may have been reinstalled or the connection may be under attack. The connection will not be established until this mismatch is explained.": "Ключ вузла %s відрізняється від раніше збереженого (отримано %s %s). Можливо, сервер перевстановлено або з'єднання підмінено. З'єднання не буде встановлено, доки причину розбіжності не з'ясовано.",
	"Host key mismatch":                                     "Ключ вузла не збігається",
	"Host key verification":                                 "Перевірка ключа вузла",
	"Host key verification is disabled":                     "Перевірку ключа вузла вимкнено",
	"Host key %s (%s %s) was saved to %s":                   "Ключ вузла %s (%s %s) збережено у %s",
	"Host key for %s was not confirmed":                     "Ключ вузла %s не підтверджено",
	"Keepalive, sec":                                        "Інтервал перевірки зв'язку, с",
	"Language":                                              "Мова",
	"Launch at system login":                                "Запускати під час входу в систему",
	"Local address %s is busy: %w":                          "Локальна адреса %s зайнята: %w",
	"Local address %s is available":                         "Локальна адреса %s вільна",
	"Local port":                                            "Локальний порт",
	"Local port must be an integer from 1 to 65535, got %d": "Локальний порт має бути цілим числом від 1 до 65535; отримано %d",
	"Local port is not set":                                 "Локальний порт не вказано",
	"Local port must be a number, got %q":                   "Локальний порт має бути числом; отримано %q",
	"Jump server":                                           "Проміжний сервер",
	"Jump server address":                                   "Адреса проміжного сервера",
	"Jump server address is not set":                        "Не вказано адресу проміжного сервера",
	"Jump server port":                                      "Порт проміжного сервера",
	"Jump server port must be an integer from 1 to 65535, got %d": "Порт проміжного сервера має бути цілим числом від 1 до 65535; отримано %d",
	"Jump server is unreachable via TCP: %w":                      "Проміжний сервер недоступний через TCP: %w",
	"Jump server did not open a channel to %s: %w":                "Проміжний сервер не відкрив канал до %s: %w",
	"Jump server %s is unreachable via TCP: %w":                   "Проміжний сервер %s недоступний через TCP: %w",
	"Jump server did not open the connection to %s: %w":           "Проміжний сервер не зміг відкрити з'єднання до %s: %w",
	"Jump server username":                                        "Ім'я користувача проміжного сервера",
	"Jump server username is not set":                             "Не вказано ім'я користувача проміжного сервера",
	"Key file":                                                    "Файл ключа",
	"Key file %s could not be decrypted: %w":                      "Не вдалося розшифрувати файл ключа %s: %w",
	"Key file %s could not be parsed: %w":                         "Не вдалося розібрати файл ключа %s: %w",
	"Key file %s could not be read: %w":                           "Не вдалося прочитати файл ключа %s: %w",
	"Key file is encrypted; a passphrase is required":             "Файл ключа зашифровано; потрібна парольна фраза",
	"Key file path is not set":                                    "Не вказано шлях до файла ключа",
	"Keep the password in the OS credential store":                "Зберігати пароль у сховищі облікових даних ОС",
	"Local port %s is occupied by another process: %w":            "Локальний порт %s зайнятий іншим процесом: %w",
	"Log":                              "Журнал",
	"Log export failed: %w":            "Не вдалося експортувати журнал: %w",
	"Log in to the system":             "Вхід до системи",
	"New profile":                      "Новий профіль",
	"No":                               "Ні",
	"No connection to the jump server": "Немає з'єднання з проміжним сервером",
	"No key found in ssh-agent; add a key with ssh-add": "В агенті SSH немає ключів; додайте ключ командою ssh-add",
	"No profile selected":                               "Профіль не вибрано",
	"No saved profiles":                                 "Збережених профілів немає",
	"Not selected":                                      "Не вибрано",
	"OK":                                                "Гаразд",
	"Open SSH-agent keys are used":                      "Використовуються ключі запущеного агента SSH",
	"Password":                                          "Пароль",
	"Password is not set":                               "Пароль не вказано",
	"Passphrase":                                        "Парольна фраза",
	"Profile %q already exists":                         "Профіль %q уже існує",
	"Profile %q deleted":                                "Профіль %q видалено",
	"Profile %q saved (password is not written to the file)": "Профіль %q збережено (пароль не записується у файл)",
	"Profile name":              "Назва профілю",
	"Profiles":                  "Профілі",
	"Profile settings":          "Налаштування профілю",
	"Save":                      "Зберегти",
	"Save application settings": "Зберегти налаштування програми",
	"Select a default profile before enabling auto-connect": "Виберіть профіль за замовчуванням, перш ніж увімкнути автоматичне підключення",
	"Settings":                        "Налаштування",
	"Settings saved":                  "Налаштування збережено",
	"Settings could not be saved: %w": "Не вдалося зберегти налаштування: %w",
	"SSH connection to jump server %s failed: %w":           "Не вдалося встановити SSH-з'єднання з проміжним сервером %s: %w",
	"SSH-agent is unavailable: %w":                          "Агент SSH недоступний: %w",
	"SSH-agent was not found: SSH_AUTH_SOCK is not set":     "Агент SSH не знайдено: змінну SSH_AUTH_SOCK не задано",
	"SSH-agent is not running; start the ssh-agent service": "Агент SSH не запущено; запустіть службу ssh-agent",
	"Start":                            "Запустити",
	"Stop":                             "Зупинити",
	"Target server":                    "Цільовий сервер",
	"Target server address":            "Адреса цільового сервера",
	"Target server address is not set": "Не вказано адресу цільового сервера",
	"Target server port":               "Порт цільового сервера",
	"Target server port must be an integer from 1 to 65535, got %d": "Порт цільового сервера має бути цілим числом від 1 до 65535; отримано %d",
	"Target server is reachable through %s via %s":                  "Цільовий сервер %s доступний через %s",
	"Test connection":                        "Перевірити з'єднання",
	"Tunnel active; connect clients to %s":   "Тунель активний; підключайте клієнтів до %s",
	"Tunnel is already running":              "Тунель уже запущено",
	"Tunnel stopped":                         "Тунель зупинено",
	"Trust":                                  "Довіряти",
	"Unknown authentication method: %q":      "Невідомий спосіб автентифікації: %q",
	"Unknown host key":                       "Невідомий ключ вузла",
	"Use the host key from the jump server?": "Довіряти ключу проміжного сервера?",
	"Username":                               "Ім'я користувача",
	"Verify the jump server host key (protects against server impersonation)": "Перевіряти ключ проміжного сервера (захист від підміни)",
	"Warning: ":  "Попередження: ",
	"Connection": "З'єднання",
	"Connection %d was rejected: no connection to the jump server":  "З'єднання %d відхилено: немає зв'язку з проміжним сервером",
	"Connection %d: jump server could not open a channel to %s: %v": "З'єднання %d: проміжний сервер не відкрив канал до %s: %v",
	"Connection %d opened: %s -> %s":                                "З'єднання %d відкрито: %s -> %s",
	"Connection %d closed: sent %d bytes, received %d bytes":        "З'єднання %d закрито: надіслано %d байтів, отримано %d байтів",
	"Could not accept a local connection: %v":                       "Не вдалося прийняти локальне з'єднання: %v",
	"Connection to the jump server was lost: %v":                    "Зв'язок із проміжним сервером втрачено: %v",
	"Connection to the jump server was lost; tunnel stopped":        "Зв'язок із проміжним сервером втрачено; тунель зупинено",
	"Connection restored; tunnel is active on %s":                   "Зв'язок відновлено; тунель активний на %s",
	"Connection check passed: %s is reachable through %s":           "Перевірку з'єднання пройдено: %s доступний через %s",
	"Connection checking":                                           "Перевірка з'єднання",
	"Establishing connection":                                       "Встановлення з'єднання",
	"Reconnecting, attempt %d":                                      "Повторне підключення, спроба %d",
	"Attempt %d failed: %v":                                         "Спроба %d невдала: %v",
	"1/3 TCP connection to jump server %s":                          "1/3 TCP-з'єднання з проміжним сервером %s",
	"2/3 SSH handshake and authentication for %s using %q":          "2/3 SSH-рукостискання й автентифікація користувача %s способом %q",
	"3/3 checking channel to target %s through jump server":         "3/3 перевірка каналу до цілі %s через проміжний сервер",
	"Connection established: %s is reachable through %s":            "З'єднання встановлено: %s доступний через %s",
	"Automatic startup failed: %w":                                  "Не вдалося налаштувати автозапуск: %w",
}

var additionalUkrainian = map[string]string{
	"Active":                             "Активний",
	"Address used by local applications": "Адреса, до якої підключаються локальні програми",
	"App settings":                       "Налаштування",
	"Application settings":               "Налаштування програми",
	"Attempt %d failed: %v":              "Спроба %d невдала: %v",
	"Back":                               "Назад",
	"Checking":                           "Перевірка",
	"Could not accept a local connection: %v":                                             "Не вдалося прийняти локальне з'єднання: %v",
	"Could not bind local address %s: %w":                                                 "Не вдалося зайняти локальну адресу %s: %w",
	"Could not connect to ssh-agent (%s): %w; start the ssh-agent service":                "Не вдалося підключитися до агента SSH (%s): %w; запустіть службу ssh-agent",
	"Could not create file %s: %w":                                                        "Не вдалося створити файл %s: %w",
	"Could not create the LaunchAgents directory: %w":                                     "Не вдалося створити каталог LaunchAgents: %w",
	"Could not create the autostart directory: %w":                                        "Не вдалося створити каталог автозапуску: %w",
	"Could not determine the application executable: %w":                                  "Не вдалося визначити виконуваний файл програми: %w",
	"Could not determine the user configuration directory: %w":                            "Не вдалося визначити каталог налаштувань користувача: %w",
	"Could not encode the executable path: %w":                                            "Не вдалося закодувати шлях до виконуваного файла: %w",
	"Could not open the current user's startup settings: %w":                              "Не вдалося відкрити налаштування автозапуску поточного користувача: %w",
	"Could not read a password from the OS credential store: %v":                          "Не вдалося прочитати пароль зі сховища облікових даних ОС: %v",
	"Could not read a password from the OS credential store: %w":                          "Не вдалося прочитати пароль зі сховища облікових даних ОС: %w",
	"Could not save a password to the OS credential store: %w":                            "Не вдалося зберегти пароль у сховищі облікових даних ОС: %w",
	"Could not delete a password from the OS credential store: %w":                        "Не вдалося видалити пароль зі сховища облікових даних ОС: %w",
	"Could not read the known-hosts list: %w":                                             "Не вдалося прочитати список відомих вузлів: %w",
	"Could not remember host key %s: %v":                                                  "Не вдалося запам'ятати ключ вузла %s: %v",
	"Could not remove the LaunchAgent: %w":                                                "Не вдалося видалити LaunchAgent: %w",
	"Could not remove the autostart entry: %w":                                            "Не вдалося видалити запис автозапуску: %w",
	"Could not remove the startup entry: %w":                                              "Не вдалося видалити запис запуску: %w",
	"Could not register the application for startup: %w":                                  "Не вдалося зареєструвати запуск програми: %w",
	"Could not restore the previous startup setting: %w":                                  "Не вдалося відновити попереднє налаштування автозапуску: %w",
	"Could not write the LaunchAgent: %w":                                                 "Не вдалося записати LaunchAgent: %w",
	"Could not write the autostart entry: %w":                                             "Не вдалося записати запис автозапуску: %w",
	"Connection %d closed: sent %d bytes, received %d bytes":                              "З'єднання %d закрито: надіслано %d байтів, отримано %d байтів",
	"Connection %d opened: %s -> %s":                                                      "З'єднання %d відкрито: %s -> %s",
	"Connection %d: jump server could not open a channel to %s: %v":                       "З'єднання %d: проміжний сервер не відкрив канал до %s: %v",
	"Connection %d was rejected: no connection to the jump server":                        "З'єднання %d відхилено: немає зв'язку з проміжним сервером",
	"Connection check passed: %s is reachable through %s":                                 "Перевірку з'єднання пройдено: %s доступний через %s",
	"Connection established: %s is reachable through %s":                                  "З'єднання встановлено: %s доступний через %s",
	"Connection restored; tunnel is active on %s":                                         "Зв'язок відновлено; тунель активний на %s",
	"Connection settings":                                                                 "Параметри з'єднання",
	"Connection to the jump server was lost: %v":                                          "Зв'язок із проміжним сервером втрачено: %v",
	"Connection to the jump server was lost; tunnel stopped":                              "Зв'язок із проміжним сервером втрачено; тунель зупинено",
	"Delete profile %q?":                                                                  "Видалити профіль %q?",
	"Edit the current profile name":                                                       "Тут можна змінити назву профілю",
	"Establishing connection":                                                             "Встановлення з'єднання",
	"File %s is corrupted: %w":                                                            "Файл %s пошкоджений: %w",
	"Host key %s (%s %s) was saved to %s":                                                 "Ключ вузла %s (%s %s) збережено у %s",
	"Host key checking is disabled; the connection is vulnerable to server impersonation": "Перевірку ключа вузла вимкнено; з'єднання вразливе до підміни сервера",
	"Host key for %s differs from the previously saved key (received %s %s). The server may have been reinstalled or the connection may be under attack. The connection will not be established until this mismatch is explained.": "Ключ вузла %s відрізняється від раніше збереженого (отримано %s %s). Можливо, сервер перевстановлено або з'єднання підмінено. З'єднання не буде встановлено, доки причину розбіжності не з'ясовано.",
	"Idle":                           "Неактивний",
	"Jump server address is not set": "Не вказано адресу проміжного сервера",
	"Jump server did not open a channel to %s: %w": "Проміжний сервер не відкрив канал до %s: %w",
	"Jump server port":                "Порт проміжного сервера",
	"Jump server username is not set": "Не вказано ім'я користувача проміжного сервера",
	"Keepalive must be a positive integer number of seconds, got %q": "Інтервал перевірки зв'язку має бути додатним цілим числом секунд; отримано %q",
	"Language":                      "Мова",
	"Local address %s is available": "Локальна адреса %s вільна",
	"Local port must be in the range 1..65535, got %d": "Локальний порт має бути в діапазоні 1..65535; отримано %d",
	"Local port must be a number, got %q":              "Локальний порт має бути числом; отримано %q",
	"Local port is not set":                            "Локальний порт не вказано",
	"New profile":                                      "Новий профіль",
	"Path to the private key file":                     "Шлях до файла приватного ключа",
	"Port":                                             "Порт",
	"Private key":                                      "Приватний ключ",
	"Password for profile %q was loaded from the OS credential store": "Пароль профілю %q завантажено зі сховища облікових даних ОС",
	"Reconnecting":                     "Повторне підключення",
	"Required if the key is encrypted": "Потрібно, якщо ключ зашифровано",
	"Select a default profile before enabling auto-connect": "Виберіть профіль за замовчуванням, перш ніж увімкнути автоматичне підключення",
	"Server %s presented a host key that has not been seen before.\n\nKey type: %s\nFingerprint: %s\n\nDo you trust this key and want to remember it?": "Сервер %s надав ключ, якого раніше не було.\n\nТип ключа: %s\nВідбиток: %s\n\nДовіряти цьому ключу й запам'ятати його?",
	"Status: %s — %s":                  "Стан: %s — %s",
	"Status: idle":                     "Стан: неактивний",
	"Target server address is not set": "Не вказано адресу цільового сервера",
	"Target server port must be an integer from 1 to 65535, got %d": "Порт цільового сервера має бути цілим числом від 1 до 65535; отримано %d",
	"The connection is established through this server":             "Через цей сервер установлюється з'єднання",
	"The jump server connects to this destination":                  "Проміжний сервер підключається до цієї адреси",
	"The selected default profile no longer exists":                 "Вибраного профілю за замовчуванням більше немає",
	"Ukrainian":                   "Українська",
	"Warning: ":                   "Попередження: ",
	"%s is not set":               "%s не вказано",
	"%s must be a number, got %q": "%s має бути числом; отримано %q",
	"%s must be in the range 1..65535, got %d":      "%s має бути в діапазоні 1..65535; отримано %d",
	"%s must be an integer from 1 to 65535, got %d": "%s має бути цілим числом від 1 до 65535; отримано %d",
}

func init() {
	activeLanguage.Store(string(English))
}

// Normalize returns a supported language, defaulting to English.
func Normalize(language string) Language {
	switch Language(language) {
	case English:
		return English
	case Ukrainian:
		return Ukrainian
	default:
		return English
	}
}

// SetLanguage changes the language used for subsequent translations.
func SetLanguage(language Language) {
	activeLanguage.Store(string(Normalize(string(language))))
}

// Current returns the active interface language.
func Current() Language {
	return Language(activeLanguage.Load().(string))
}

// T translates an English source phrase into the active language.
func T(english string) string {
	if Current() == Ukrainian {
		if translated, ok := additionalUkrainian[english]; ok {
			return translated
		}
		if translated, ok := ukrainian[english]; ok {
			return translated
		}
	}
	return english
}
