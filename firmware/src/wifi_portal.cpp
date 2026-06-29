#include "wifi_portal.h"
#include "display.h"
#include "mascot.h"
#include "layout.h"
#include "elf_wifi.h"
#include <WiFi.h>
#include <WebServer.h>
#include <DNSServer.h>
#include <M5Unified.h>
#include <map>

static const char* kPortalSsid = "Anya";
static const byte kDnsPort = 53;

// ── Portal HTML (adapted from Arkloop style) ─────────────────
static const char kPortalHtml[] PROGMEM = R"HTML(
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Anya WiFi Setup</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%;overflow:hidden}
body{font-family:-apple-system,sans-serif;background:#111;color:#eee;display:flex;flex-direction:column;max-width:400px;margin:0 auto}
h1{font-size:1.3em;text-align:center;padding:16px 20px 8px;flex-shrink:0;position:relative}
#rescan{position:absolute;right:20px;top:14px;width:32px;height:32px;border-radius:50%;border:1px solid #444;background:#222;color:#eee;font-size:16px;cursor:pointer;display:flex;align-items:center;justify-content:center;padding:0;line-height:1}
#rescan:active{background:#333}
#rescan.spin{animation:spin .6s linear}
@keyframes spin{to{transform:rotate(360deg)}}
#top-area{flex:1;overflow-y:auto;padding:0 20px;-webkit-overflow-scrolling:touch}
.scanning{text-align:center;padding:40px 0;color:#888}
#list{display:none}
.wifi-item{display:flex;align-items:center;gap:12px;padding:12px 14px;margin-bottom:4px;border-radius:10px;background:#1a1a1a;cursor:pointer;border:1px solid transparent;transition:border-color .15s}
.wifi-item:hover,.wifi-item.sel{border-color:#fff}
.wifi-item .sig{font-family:monospace;font-size:1.2em;letter-spacing:-2px;min-width:44px}
.wifi-item .sig.s4{color:#4ade80}.wifi-item .sig.s3{color:#a3e635}.wifi-item .sig.s2{color:#facc15}.wifi-item .sig.s1{color:#f97316}.wifi-item .sig.s0{color:#ef4444}
.wifi-item .info{flex:1;min-width:0}
.wifi-item .ssid{font-size:.95em;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.wifi-item .lock{color:#888;font-size:.8em}
#bottom-bar{flex-shrink:0;padding:12px 20px 16px;background:#111;border-top:1px solid #222}
#bottom-bar input,#bottom-bar button{width:100%;padding:11px;font-size:1em;border-radius:8px;margin-bottom:8px}
#bottom-bar input{background:#222;color:#eee;border:1px solid #444}
#bottom-bar input:disabled{opacity:.4}
#bottom-bar button{border:none;font-weight:600;cursor:pointer}
#bottom-bar button.primary{background:#fff;color:#111}
#bottom-bar button:disabled{opacity:.5}
#status{text-align:center;font-size:.85em;padding:4px 20px 0;flex-shrink:0}
#status.error{color:#f87171}
#status.ok{color:#4ade80}
</style></head>
<body>
<h1>Choose WiFi<button id="rescan" onclick="reScan()">&#x21bb;</button></h1>
<div id="status"></div>
<div id="top-area">
<div id="scanning" class="scanning">Scanning networks...</div>
<div id="list"></div>
</div>
<div id="bottom-bar">
<input type="password" id="password" placeholder="Password" disabled>
<button id="connect" class="primary" disabled>Connect</button>
</div>
<script>
function sigBars(r){return r>=-50?4:r>=-60?3:r>=-70?2:r>=-80?1:0}
function sigClass(r){return's'+sigBars(r)}
var nets=[],selSsid='',selSecure=false;
async function scan(){try{var r=await fetch('/scan');nets=(await r.json()).sort(function(a,b){return b.rssi-a.rssi});var l=document.getElementById('list');l.innerHTML='';nets.forEach(function(n){var d=document.createElement('div');d.className='wifi-item';d.onclick=function(){selectNetwork(n.ssid,n.secure);document.querySelectorAll('.wifi-item').forEach(function(e){e.classList.toggle('sel',e===d)})};var bars=['▂','▄','▆','█'];var b=bars.slice(0,sigBars(n.rssi)).join('');d.innerHTML='<span class="sig '+sigClass(n.rssi)+'">'+b+'</span><div class="info"><div class="ssid">'+n.ssid+'</div></div>'+(n.secure?'<span class="lock">&#x1f512;</span>':'');l.appendChild(d)});document.getElementById('scanning').style.display='none';l.style.display='block'}catch(e){document.getElementById('scanning').textContent='Scan failed. Tap Rescan.'}}
function selectNetwork(ssid,secure){selSsid=ssid;selSecure=secure;var p=document.getElementById('password');if(secure){p.disabled=false;p.placeholder='Password';p.value=''}else{p.disabled=true;p.placeholder='Open network - no password';p.value=''}document.getElementById('connect').disabled=false}
document.getElementById('connect').addEventListener('click',async function(){var p=document.getElementById('password').value;if(!selSsid)return;if(selSecure&&!p){var st=document.getElementById('status');st.textContent='Please enter WiFi password';st.className='error';return}var st=document.getElementById('status');var b=document.getElementById('connect');b.disabled=true;st.textContent='Connecting...';st.className='';try{var r=await fetch('/connect',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:'ssid='+encodeURIComponent(selSsid)+'&password='+encodeURIComponent(p)});var j=await r.json();if(j.ok){st.textContent='Connected! Device will continue...';st.className='ok'}else{st.textContent='Error: '+j.error;st.className='error';b.disabled=false}}catch(e){st.textContent='Connection failed';st.className='error';b.disabled=false}});
function reScan(){var s=document.getElementById('scanning');s.style.display='';s.textContent='Scanning networks...';document.getElementById('list').style.display='none';document.getElementById('rescan').classList.add('spin');selSsid='';document.getElementById('connect').disabled=true;document.getElementById('password').disabled=true;document.getElementById('password').value='';scan().finally(function(){document.getElementById('rescan').classList.remove('spin')})}
scan();
</script>
</body>
</html>
)HTML";

// ── JSON escaping ───────────────────────────────────────────
static String jsonEscape(const String& s) {
    String out;
    out.reserve(s.length() + 4);
    for (size_t i = 0; i < s.length(); i++) {
        char c = s[i];
        switch (c) {
            case '"':  out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            default:   out += c;
        }
    }
    return out;
}

// ── Scan with SSID dedup (keep strongest RSSI) ─────────────
static String buildScanJson() {
    int n = WiFi.scanNetworks(false, false, false, 300);
    std::map<String, int> bestRssi;
    std::map<String, bool> isSecure;
    for (int i = 0; i < n; i++) {
        String ssid = WiFi.SSID(i);
        if (ssid.length() == 0) continue;
        int rssi = WiFi.RSSI(i);
        auto it = bestRssi.find(ssid);
        if (it == bestRssi.end() || rssi > it->second) {
            bestRssi[ssid] = rssi;
            isSecure[ssid] = WiFi.encryptionType(i) != WIFI_AUTH_OPEN;
        }
    }
    WiFi.scanDelete();

    String json = "[";
    bool first = true;
    for (const auto& entry : bestRssi) {
        if (!first) json += ",";
        first = false;
        json += "{\"ssid\":\"" + jsonEscape(entry.first) +
                "\",\"rssi\":" + String(entry.second) +
                ",\"secure\":" + String(isSecure[entry.first] ? "true" : "false") + "}";
    }
    json += "]";
    return json;
}

// ── Screen drawing (135×240 portrait, matches display.cpp) ──
static const int P_LINE_SPACING = 18;
static int portalMascotY = 0;   // computed at runtime after M5.begin()
static int portalPromptY = 0;
static bool portalDrawn = false;
static int  portalFrame  = 0;
static int  lastPortalDrawnFrame = -1;
static unsigned long portalFrameStart = 0;

static void portalDrawStatusBar() {
    M5.Display.fillRect(0, 0, M5.Display.width(), STATUS_BAR_H, TFT_BLACK);
    M5.Display.drawFastHLine(0, STATUS_BAR_H, M5.Display.width(), TFT_DARKGREY);

    // Connection dot (portal mode = not connected)
    M5.Display.drawCircle(6, STATUS_BAR_H / 2, 3, TFT_DARKGREY);

    // Title centred in the status bar
    M5.Display.setTextSize(1);
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextDatum(textdatum_t::middle_center);
    M5.Display.drawString("Anya", M5.Display.width() / 2, STATUS_BAR_H / 2);
}

static void portalCenterPrint(const char* s, int y) {
    M5.Display.setTextColor(TFT_WHITE);
    M5.Display.setTextSize(1);
    int w = strlen(s) * 6;
    M5.Display.setCursor((M5.Display.width() - w) / 2, y);
    M5.Display.print(s);
}

static void updatePortalLayout() {
    computeMascotLayout(M5.Display.height(), MASCOT_IMG_H, portalMascotY, portalPromptY);
}

static void portalDrawMascot() {
    unsigned long now = millis();
    if (portalFrameStart == 0) portalFrameStart = now;

    if (now - portalFrameStart >= MASCOT_FRAME_DURATIONS[portalFrame]) {
        portalFrame = (portalFrame + 1) % MASCOT_FRAMES;
        portalFrameStart = now;
    }

    if (portalFrame == lastPortalDrawnFrame) return;
    lastPortalDrawnFrame = portalFrame;

    int x = (M5.Display.width() - MASCOT_IMG_W) / 2;
    mascot_draw(portalFrame, x, portalMascotY);
}

static void drawPortalScreen() {
    M5.Display.setRotation(DISPLAY_ROTATION);
    updatePortalLayout();

    if (!portalDrawn) {
        M5.Display.fillScreen(TFT_BLACK);
        M5.Display.setBrightness(255);
        portalDrawStatusBar();
        portalCenterPrint("Connect to Anya", portalPromptY);
        portalDrawn = true;
    }
    portalDrawMascot();  // animate every call
}

static void drawConnectingScreen(const char* ssid) {
    int x = (M5.Display.width() - MASCOT_IMG_W) / 2;

    M5.Display.fillScreen(TFT_BLACK);
    M5.Display.setBrightness(255);
    updatePortalLayout();
    portalDrawStatusBar();
    lastPortalDrawnFrame = -1;  // force redraw after screen clear
    mascot_draw(portalFrame, x, portalMascotY);
    portalCenterPrint("Connecting...", portalPromptY);
}

// ── Main portal loop ────────────────────────────────────────
bool wifi_portal_begin() {
    WiFi.mode(WIFI_AP);
    if (!WiFi.softAP(kPortalSsid)) return false;

    WebServer server(80);
    DNSServer dns;
    bool submitted = false;
    String submittedSsid;
    String submittedPass;

    IPAddress apIP = WiFi.softAPIP();
    dns.start(kDnsPort, "*", apIP);

    server.on("/", [&]() { server.send_P(200, "text/html", kPortalHtml); });
    server.on("/scan", [&]() { server.send(200, "application/json", buildScanJson()); });
    server.on("/connect", HTTP_POST, [&]() {
        submittedSsid = server.arg("ssid");
        submittedPass = server.arg("password");
        if (submittedSsid.length() == 0) {
            server.send(400, "application/json", "{\"ok\":false,\"error\":\"ssid is required\"}");
            return;
        }
        wifi_save_credentials(submittedSsid.c_str(), submittedPass.c_str());
        submitted = true;
        server.send(200, "application/json", "{\"ok\":true}");
    });
    server.onNotFound([&]() { server.send_P(200, "text/html", kPortalHtml); });
    server.begin();

    while (!submitted) {
        server.handleClient();
        for (int i = 0; i < 8; ++i) {
            dns.processNextRequest();
        }
        drawPortalScreen();
        delay(10);
    }

    server.stop();
    dns.stop();
    WiFi.softAPdisconnect(true);
    delay(200);
    drawConnectingScreen(submittedSsid.c_str());
    return wifi_init();
}
