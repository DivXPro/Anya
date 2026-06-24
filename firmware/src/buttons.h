#pragma once
#include <functional>

typedef std::function<void()> ButtonCallback;

void btn_init();
void btn_loop();
void btn_on_ptt_press(ButtonCallback cb);
void btn_on_ptt_release(ButtonCallback cb);
void btn_on_confirm(ButtonCallback cb);
void btn_on_next(ButtonCallback cb);
