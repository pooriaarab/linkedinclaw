// LinkedInClaw Companion MV3 Extension
// Passively captures the LinkedIn session cookies and POSTs them to the local linkedinclaw listener.

const LINKEDIN_URL = "https://www.linkedin.com";
const TARGET_DOMAIN = "www.linkedin.com";
const PORT = 9090; // Default port used by linkedinclaw listener
const SESSION_ENDPOINT = `http://127.0.0.1:${PORT}/session`;

interface SessionPayload {
  li_at: string;
  jsessionid: string;
}

// Track previous values to avoid duplicate requests when nothing has changed
let lastLiAt: string | null = null;
let lastJSessionID: string | null = null;

chrome.cookies.onChanged.addListener((changeInfo) => {
  const { cookie, removed } = changeInfo;

  // Only listen to cookies from www.linkedin.com
  if (cookie.domain !== TARGET_DOMAIN) {
    return;
  }

  // Only trigger on li_at or JSESSIONID cookies
  if (cookie.name !== "li_at" && cookie.name !== "JSESSIONID") {
    return;
  }

  // If cookie was removed, we don't want to propagate empty values
  if (removed) {
    return;
  }

  // Query both cookies to ensure we send the latest complete pair
  Promise.all([
    getCookie("li_at"),
    getCookie("JSESSIONID")
  ]).then(([liAt, jsessionid]) => {
    if (liAt && jsessionid) {
      // Avoid duplicate network requests if the cookies haven't changed from our cached values
      if (liAt === lastLiAt && jsessionid === lastJSessionID) {
        return;
      }

      lastLiAt = liAt;
      lastJSessionID = jsessionid;

      sendSessionToBackend(liAt, jsessionid);
    }
  }).catch((err) => {
    // Fail silently per design spec
  });
});

function getCookie(name: string): Promise<string | null> {
  return new Promise((resolve) => {
    chrome.cookies.get({ url: LINKEDIN_URL, name }, (cookie) => {
      if (chrome.runtime.lastError) {
        resolve(null);
        return;
      }
      resolve(cookie ? cookie.value : null);
    });
  });
}

async function sendSessionToBackend(liAt: string, jsessionid: string) {
  const payload: SessionPayload = {
    li_at: liAt,
    jsessionid: jsessionid
  };

  try {
    const response = await fetch(SESSION_ENDPOINT, {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify(payload)
    });

    if (!response.ok) {
      // If endpoint returns non-OK status, fail silently
    }
  } catch (error) {
    // Fail silently and retry on next cookie-change event if listener is not running
  }
}
