San Tower
=========

A workflow tower with a node editor: triggers (schedule, Telegram, events),
steps for data, files and HTTP, and steps that post or ask through your own
browser (Threads, X, Facebook, Medium, LinkedIn, TikTok, Vidara, ChatGPT,
Gemini, Claude). Everything runs on this computer only.

What is in this folder
- san_tower.exe (Windows) or san_tower (Linux)   the tower
- start.bat (Windows)                             starts it
- san_remote_extension/                           the browser extension the
                                                  posting and asking steps use

1. Start the tower
   Windows: double-click start.bat, or in a terminal in this folder:
       san_tower.exe serve
   Linux, in a terminal in this folder:
       ./san_tower serve
   Then open http://127.0.0.1:7477 in your browser.
   Windows may warn that the app is unknown: More info > Run anyway.
   Its data (workflows, connections, storages) goes in a "data" folder
   where you start it. Keep it running while you use the tower.

2. Turn on the Browser Remote Server
   In the tower, open the Browser Remote Server page and switch it on.
   It stays on the next time the tower starts.

3. Install the extension (Chrome or Edge, version 135 or newer)
   - Open chrome://extensions (Edge: edge://extensions).
   - Turn on Developer mode.
   - Load unpacked, and pick the san_remote_extension folder from here.
   - Open the extension's Details and turn on "Allow User Scripts".
   - Click the extension's icon: it should say it is connected. Its Server
     should be http://127.0.0.1:7476.
   Leave the folder where it is: the browser loads the extension from it.

4. Add a connection
   In the tower, Connections > add a San Remote connection with the URL
   http://127.0.0.1:7478, then press Test: it should list your browser.
   Steps that post or ask pick this connection.

5. Sign in
   Sign in to the sites in that same browser. The steps click through the
   sites as a person does, so X, Threads, Medium, LinkedIn, TikTok, Vidara,
   ChatGPT, Gemini and Claude must show their pages in English, and
   Facebook in Indonesian.

Notes
- Posting steps only schedule posts; they bring the browser to the front
  for a minute or two. If its taskbar button only flashes, click it.
- Claude Ask (CLI) needs Claude Code installed and signed in.
- Telegram steps need a bot token from @BotFather (Connections > Telegram Bot).
