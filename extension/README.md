# LinkedInClaw Companion Extension (MV3)

A lightweight Chrome Companion Extension (Manifest V3) for **linkedinclaw** that passively refreshes your LinkedIn session credentials (`li_at` and `JSESSIONID` cookies) to keep your local mirroring CLI authenticated.

## What It Does
- **Passive Cookie Capture**: Listens to cookie change events on `www.linkedin.com` scoped *exclusively* to `li_at` and `JSESSIONID`.
- **Local Synchronization**: Reads current values of these two cookies via secure `chrome.cookies` APIs and POSTs them strictly to your localhost linkedinclaw listener (`http://127.0.0.1:9090/session`).
- **Silent Retry**: If the linkedinclaw CLI listener isn't running when a cookie changes, it fails silently and will try again on the next cookie change event without disrupting your browser experience.

## What It DOES NOT Do
- **No Page Scraping / DOM Access**: It does not read your LinkedIn feed, messages, profiles, connections, or inject scripts into your page. It runs entirely in the background.
- **No Remote Network Communication**: It does not send any data to remote servers or third-party APIs. It only makes requests to `http://127.0.0.1` (localhost).
- **No Storage/Logging**: It does not persist or log your cookies anywhere. It acts as a passive bridge.

## Installation Steps (Developer / Unpacked Mode)

Since this is a custom local companion, you can load it unpacked directly in your Chromium-based browser (Chrome, Brave, Edge, etc.):

### 1. Build the Script
To build the background service worker script, compile `background.ts` to `background.js` in the `extension/` directory.

Since it has no external dependencies beyond browser APIs, you can compile it using any standard TypeScript compiler:
```bash
# From the project root or extension/ folder:
npx tsc extension/background.ts --target es2020 --module commonjs --allowJs false
```
*Note: Ensure the output file is named `background.js` and sits in the `extension/` folder next to `manifest.json`.*

### 2. Load the Extension
1. Open your browser and navigate to `chrome://extensions/`.
2. Enable **Developer mode** (usually a toggle in the top-right corner).
3. Click the **Load unpacked** button in the top-left.
4. Select the `extension/` directory of this repository (the folder containing `manifest.json` and `background.js`).

### 3. Verify
1. Ensure the `linkedinclaw` CLI listener is running (typically started via `linkedinclaw`'s authentication or serving commands).
2. Go to [linkedin.com](https://www.linkedin.com) and log in, or refresh your existing session.
3. The extension will automatically detect the cookies and securely transmit them to your running linkedinclaw listener.
