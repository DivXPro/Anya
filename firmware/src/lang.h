#pragma once

#include <cstdint>

// Supported languages. English is the default.
enum class Lang : uint8_t {
    EN = 0,
    ZH = 1,
};

// String identifiers for all user-facing text.
enum class Str : uint8_t {
    ClickToSpeak,
    Disconnected,
    ReadyToPair,
    Pairing,
    Connecting,
    Listening,
    Sending,
    Thinking,
    NoAgent,
    ConnectToAnya,
    UpdatingFirmware,
    WifiSetupFailed,
    MenuChooseWifi,
    MenuRepair,
    MenuTestSpeaker,
    MenuBack,
    MenuLanguage,
    LangEnglish,
    LangChinese,
    PlayingTestTone,
    Count
};

// Load persisted language (defaults to English).
void lang_init();

// Get/set the current language. lang_set persists the choice.
Lang lang_get();
void lang_set(Lang lang);

// Translate a string ID to the current language.
const char* tr(Str id);
