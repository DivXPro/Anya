#include "wifi_portal.h"
#include "wifi.h"
#include <M5Unified.h>
#include <WiFi.h>
#include <WebServer.h>
#include <DNSServer.h>

static const char* kPortalSsid = "Elf-hotspot";
static const byte kDnsPort = 53;

static const char kPortalHtml[] PROGMEM = R"HTML(
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Elf WiFi Setup</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#111;color:#f5f5f5;margin:0;padding:24px;max-width:420px}
h1{font-size:24px;margin:0 0 8px}.hint{color:#aaa;margin-bottom:20px}
.wifi{width:100%;text-align:left;border:1px solid #333;background:#1b1b1b;color:#fff;border-radius:12px;padding:12px;margin:6px 0}
.wifi.sel{border-color:#fff}input,button{box-sizing:border-box;width:100%;border-radius:12px;padding:12px;margin:8px 0;font-size:16px}
input{background:#1b1b1b;color:#fff;border:1px solid #444}button{border:0;background:#fff;color:#111;font-weight:700}
#status{min-height:24px;color:#9ae6b4}
</style>
</head>
<body>
<h1>Elf WiFi Setup</h1>
<div class="hint">Choose a WiFi network for your Elf device.</div>
<div id="list">Scanning...</div>
<input id="password" type="password" placeholder="WiFi password">
<button id="connect" disabled>Connect</button>
<div id="status"></div>
<script>
let selected='';
async function scan(){const r=await fetch('/scan');const nets=await r.json();const list=document.getElementById('list');list.innerHTML='';nets.forEach(n=>{const b=document.createElement('button');b.className='wifi';b.textContent=n.ssid+'  '+n.rssi+'dBm'+(n.secure?' locked':' open');b.onclick=()=>{selected=n.ssid;document.querySelectorAll('.wifi').forEach(x=>x.classList.remove('sel'));b.classList.add('sel');document.getElementById('connect').disabled=false};list.appendChild(b)})}
document.getElementById('connect').onclick=async()=>{const body=new URLSearchParams({ssid:selected,password:document.getElementById('password').value});document.getElementById('status').textContent='Connecting...';const r=await fetch('/connect',{method:'POST',body});const j=await r.json();document.getElementById('status').textContent=j.ok?'Connected. Device will continue.':'Error: '+j.error};
scan();
</script>
</body>
</html>
)HTML";

static void drawPortalScreen(const char* ssid) {
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.fillCircle(80, 40, 32, TFT_LIGHTGREY);
    M5.Display.setCursor(22, 92);
    M5.Display.print("Connect ");
    M5.Display.print(ssid);
    M5.Display.setCursor(42, 116);
    M5.Display.print("and setup");
}

static void drawConnectingScreen(const char* ssid) {
    M5.Display.fillScreen(TFT_WHITE);
    M5.Display.setTextColor(TFT_BLACK);
    M5.Display.setTextSize(2);
    M5.Display.fillCircle(80, 40, 32, TFT_LIGHTGREY);
    M5.Display.setCursor(18, 98);
    M5.Display.print("Connecting WiFi");
}

static String jsonEscape(const String& s) {
    String out;
    for (size_t i = 0; i < s.length(); i++) {
        char c = s[i];
        if (c == '"' || c == '\\') out += '\\';
        out += c;
    }
    return out;
}

static String buildScanJson() {
    int n = WiFi.scanNetworks(false, false, false, 300);
    String json = "[";
    bool first = true;
    for (int i = 0; i < n; i++) {
        String ssid = WiFi.SSID(i);
        if (ssid.length() == 0) continue;
        if (!first) json += ",";
        first = false;
        json += "{\"ssid\":\"" + jsonEscape(ssid) + "\",\"rssi\":" + String(WiFi.RSSI(i)) +
                ",\"secure\":" + String(WiFi.encryptionType(i) == WIFI_AUTH_OPEN ? "false" : "true") + "}";
    }
    json += "]";
    WiFi.scanDelete();
    return json;
}

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
        dns.processNextRequest();
        drawPortalScreen(kPortalSsid);
        delay(10);
    }

    server.stop();
    dns.stop();
    WiFi.softAPdisconnect(true);
    delay(200);
    drawConnectingScreen(submittedSsid.c_str());
    return wifi_init();
}
