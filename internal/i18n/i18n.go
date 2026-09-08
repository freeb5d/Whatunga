// Package i18n provides simple key-based translation for Whatunga's
// web UI, in English, Te Reo Māori, French, and Chinese (Simplified).
//
// This is intentionally a plain map-based lookup, not a full ICU/CLDR
// pluralization system — the UI only needs flat labels, not sentences
// with numeric agreement, so the extra complexity wouldn't earn its
// keep here.
package i18n

// Language describes one supported UI language for the language switcher.
type Language struct {
	Code string
	Name string // shown in its own language, e.g. "Te Reo Māori"
}

// SupportedLanguages lists every language the UI can render, in the
// order they should appear in the switcher.
func SupportedLanguages() []Language {
	return []Language{
		{Code: "en", Name: "English"},
		{Code: "mi", Name: "Te Reo Māori"},
		{Code: "fr", Name: "Français"},
		{Code: "zh", Name: "中文"},
	}
}

// IsSupported reports whether code is one of the languages we have
// translations for.
func IsSupported(code string) bool {
	for _, l := range SupportedLanguages() {
		if l.Code == code {
			return true
		}
	}
	return false
}

// T returns the translation for key in the given language, falling
// back to English and then to the raw key itself if no translation
// is found — so a missing string shows up as visibly wrong text
// rather than a blank space or a panic.
func T(lang, key string) string {
	if table, ok := translations[lang]; ok {
		if value, ok := table[key]; ok {
			return value
		}
	}
	if value, ok := translations["en"][key]; ok {
		return value
	}
	return key
}

var translations = map[string]map[string]string{
	"en": {
		"app_name":                  "Whatunga",
		"tagline":                   "Simple RouterOS monitoring.",
		"nav_dashboard":             "Dashboard",
		"nav_account":               "Account",
		"nav_logout":                "Log out",
		"login_title":               "Sign in",
		"username":                  "Username",
		"password":                  "Password",
		"login_button":              "Sign in",
		"invalid_credentials":       "Incorrect username or password.",
		"change_password_title":     "Change password",
		"current_password":         "Current password",
		"new_password":              "New password",
		"confirm_password":          "Confirm new password",
		"update_button":             "Update password",
		"password_updated":          "Password updated successfully.",
		"password_mismatch":         "New password and confirmation do not match.",
		"incorrect_current_password": "Current password is incorrect.",
		"password_too_short":        "New password must be at least 8 characters.",
		"device":                    "Device",
		"devices":                   "Devices",
		"cpu_load":                  "CPU load",
		"uptime":                    "Uptime",
		"version":                   "RouterOS version",
		"board_name":                "Board",
		"interfaces":                "Interfaces",
		"running":                   "Running",
		"stopped":                   "Stopped",
		"no_devices":                "No device data yet — waiting for the first poll.",
		"welcome":                   "Signed in as",
		"language":                  "Language",
		"last_checked":              "Last checked",
		"rx":                        "Received",
		"tx":                        "Transmitted",
		"back_to_dashboard":         "Back to dashboard",
		"health":                    "Health",
		"power_state":               "Power state",
		"memory":                    "Memory",
		"kind_routeros":             "MikroTik RouterOS",
		"kind_ilo":                  "HPE iLO",
		"kind_windows":              "Windows Server",
		"kind_snmp_firewall":        "Firewall (SNMP)",
		"status_page_title":        "System Status",
		"status_page_subtitle":     "Live status of monitored devices.",
		"status_up":                "Operational",
		"status_down":              "Down",
		"status_unknown":           "Unknown",
	},
	"mi": {
		"app_name":                  "Whatunga",
		"tagline":                   "He aroturuki RouterOS ngāwari.",
		"nav_dashboard":             "Papa Mataaho",
		"nav_account":               "Pūkete",
		"nav_logout":                "Takiputa",
		"login_title":               "Takiuru",
		"username":                  "Ingoa Kaiwhakamahi",
		"password":                  "Kupuhipa",
		"login_button":              "Takiuru",
		"invalid_credentials":       "He hē te ingoa kaiwhakamahi, te kupuhipa rānei.",
		"change_password_title":     "Whakarerekē kupuhipa",
		"current_password":         "Kupuhipa o nāianei",
		"new_password":              "Kupuhipa hou",
		"confirm_password":          "Whakaū kupuhipa hou",
		"update_button":             "Whakahou kupuhipa",
		"password_updated":          "Kua whakahoutia te kupuhipa.",
		"password_mismatch":         "Kāore e rite ana te kupuhipa hou me te whakaūnga.",
		"incorrect_current_password": "He hē te kupuhipa o nāianei.",
		"password_too_short":        "Me neke atu i te 8 pūāhua te kupuhipa hou.",
		"device":                    "Pūrere",
		"devices":                   "Ngā Pūrere",
		"cpu_load":                  "Taumaha CPU",
		"uptime":                    "Wā Oranga",
		"version":                  "Putanga RouterOS",
		"board_name":                "Papa",
		"interfaces":                "Ngā Atanga",
		"running":                   "E Rere Ana",
		"stopped":                   "Kua Mutu",
		"no_devices":                "Kāore anō he raraunga — e tatari ana mō te pae tuatahi.",
		"welcome":                   "Kua takiuru ko",
		"language":                  "Reo",
		"last_checked":              "Tirohia whakamutunga",
		"rx":                        "Kua Riro Mai",
		"tx":                        "Kua Tukuna",
		"back_to_dashboard":         "Hoki ki te Papa Mataaho",
		"health":                    "Hauora",
		"power_state":               "Tūnga Hiko",
		"memory":                    "Pūmahara",
		"kind_routeros":             "MikroTik RouterOS",
		"kind_ilo":                  "HPE iLO",
		"kind_windows":              "Windows Server",
		"kind_snmp_firewall":        "Ahi Taiapa (SNMP)",
		"status_page_title":        "Tūnga Pūnaha",
		"status_page_subtitle":     "Te tūnga ora o ngā pūrere e arotakengia ana.",
		"status_up":                "E Mahi Ana",
		"status_down":              "Kua Hinga",
		"status_unknown":           "Kāore e Mōhiotia",
	},
	"fr": {
		"app_name":                  "Whatunga",
		"tagline":                   "Surveillance RouterOS simplifiée.",
		"nav_dashboard":             "Tableau de bord",
		"nav_account":               "Compte",
		"nav_logout":                "Déconnexion",
		"login_title":               "Connexion",
		"username":                  "Nom d'utilisateur",
		"password":                  "Mot de passe",
		"login_button":              "Se connecter",
		"invalid_credentials":       "Nom d'utilisateur ou mot de passe incorrect.",
		"change_password_title":     "Changer le mot de passe",
		"current_password":         "Mot de passe actuel",
		"new_password":              "Nouveau mot de passe",
		"confirm_password":          "Confirmer le nouveau mot de passe",
		"update_button":             "Mettre à jour",
		"password_updated":          "Mot de passe mis à jour avec succès.",
		"password_mismatch":         "Le nouveau mot de passe et la confirmation ne correspondent pas.",
		"incorrect_current_password": "Le mot de passe actuel est incorrect.",
		"password_too_short":        "Le nouveau mot de passe doit contenir au moins 8 caractères.",
		"device":                    "Appareil",
		"devices":                   "Appareils",
		"cpu_load":                  "Charge CPU",
		"uptime":                    "Disponibilité",
		"version":                   "Version RouterOS",
		"board_name":                "Carte",
		"interfaces":                "Interfaces",
		"running":                   "En marche",
		"stopped":                   "Arrêtée",
		"no_devices":                "Aucune donnée pour le moment — en attente du premier sondage.",
		"welcome":                   "Connecté en tant que",
		"language":                  "Langue",
		"last_checked":              "Dernière vérification",
		"rx":                        "Reçu",
		"tx":                        "Transmis",
		"back_to_dashboard":         "Retour au tableau de bord",
		"health":                    "État de santé",
		"power_state":               "État d'alimentation",
		"memory":                    "Mémoire",
		"kind_routeros":             "MikroTik RouterOS",
		"kind_ilo":                  "HPE iLO",
		"kind_windows":              "Windows Server",
		"kind_snmp_firewall":        "Pare-feu (SNMP)",
		"status_page_title":        "État du système",
		"status_page_subtitle":     "État en direct des appareils surveillés.",
		"status_up":                "Opérationnel",
		"status_down":              "En panne",
		"status_unknown":           "Inconnu",
	},
	"zh": {
		"app_name":                  "Whatunga 网络看护",
		"tagline":                   "简易 RouterOS 监控工具。",
		"nav_dashboard":             "仪表盘",
		"nav_account":               "账户",
		"nav_logout":                "退出登录",
		"login_title":               "登录",
		"username":                  "用户名",
		"password":                  "密码",
		"login_button":              "登录",
		"invalid_credentials":       "用户名或密码不正确。",
		"change_password_title":     "修改密码",
		"current_password":         "当前密码",
		"new_password":              "新密码",
		"confirm_password":          "确认新密码",
		"update_button":             "更新密码",
		"password_updated":          "密码更新成功。",
		"password_mismatch":         "新密码与确认密码不一致。",
		"incorrect_current_password": "当前密码不正确。",
		"password_too_short":        "新密码长度至少需要 8 个字符。",
		"device":                    "设备",
		"devices":                   "设备列表",
		"cpu_load":                  "CPU 负载",
		"uptime":                    "运行时间",
		"version":                  "RouterOS 版本",
		"board_name":                "主板",
		"interfaces":                "网络接口",
		"running":                   "运行中",
		"stopped":                   "已停止",
		"no_devices":                "暂无数据 — 等待首次轮询。",
		"welcome":                   "已登录：",
		"language":                  "语言",
		"last_checked":              "最后检查时间",
		"rx":                        "接收",
		"tx":                        "发送",
		"back_to_dashboard":         "返回仪表盘",
		"health":                    "健康状态",
		"power_state":               "电源状态",
		"memory":                    "内存",
		"kind_routeros":             "MikroTik RouterOS",
		"kind_ilo":                  "HPE iLO",
		"kind_windows":              "Windows 服务器",
		"kind_snmp_firewall":        "防火墙（SNMP）",
		"status_page_title":        "系统状态",
		"status_page_subtitle":     "受监控设备的实时状态。",
		"status_up":                "运行正常",
		"status_down":              "已宕机",
		"status_unknown":           "未知",
	},
}
