#include "lang.h"
#include <Preferences.h>

static const char* kPrefsNamespace = "elf-lang";
static const char* kPrefsKey = "lang";

static Lang s_current = Lang::EN;

// Translation table: [Str index][Lang index]
static const char* s_strings[static_cast<int>(Str::Count)][2] = {
    /* ClickToSpeak    */ {"Click to speak", "点击说话"},
    /* Disconnected    */ {"Disconnected", "已断开"},
    /* ReadyToPair     */ {"Ready to pair", "等待配对"},
    /* Pairing         */ {"Pairing...", "配对中..."},
    /* Connecting      */ {"Connecting...", "连接中..."},
    /* Listening       */ {"Listening...", "聆听中..."},
    /* Sending         */ {"Sending...", "发送中..."},
    /* Thinking        */ {"Thinking...", "思考中..."},
    /* NoAgent         */ {"No agent", "未连接"},
    /* ConnectToAnya   */ {"Connect to Anya", "连接 Anya"},
    /* UpdatingFirmware*/ {"Updating firmware...", "升级固件..."},
    /* WifiSetupFailed */ {"WiFi setup failed", "WiFi 设置失败"},
    /* MenuChooseWifi  */ {"Choose WiFi", "选择 WiFi"},
    /* MenuRepair      */ {"Repair", "重新配对"},
    /* MenuTestSpeaker */ {"Test Speaker", "测试扬声器"},
    /* MenuBack        */ {"Back", "返回"},
    /* MenuLanguage    */ {"Language", "语言"},
    /* LangEnglish     */ {"English", "English"},
    /* LangChinese     */ {"中文", "中文"},
    /* PlayingTestTone */ {"Playing test tone...", "播放测试音..."},
    /* NoResponse      */ {"No response, tap to retry", "无响应,点击重试"},
};

void lang_init() {
    Preferences prefs;
    if (prefs.begin(kPrefsNamespace, true)) {
        uint8_t value = prefs.getUChar(kPrefsKey, static_cast<uint8_t>(Lang::EN));
        prefs.end();
        if (value <= static_cast<uint8_t>(Lang::ZH)) {
            s_current = static_cast<Lang>(value);
        }
    }
}

Lang lang_get() {
    return s_current;
}

void lang_set(Lang lang) {
    s_current = lang;
    Preferences prefs;
    if (prefs.begin(kPrefsNamespace, false)) {
        prefs.putUChar(kPrefsKey, static_cast<uint8_t>(lang));
        prefs.end();
    }
}

const char* tr(Str id) {
    int idx = static_cast<int>(id);
    if (idx < 0 || idx >= static_cast<int>(Str::Count)) {
        return "";
    }
    return s_strings[idx][static_cast<int>(s_current)];
}
