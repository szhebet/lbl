package app.library.twa;

import android.app.Activity;
import android.app.DownloadManager;
import android.content.Context;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.util.Log;
import android.view.Gravity;
import android.view.View;
import java.io.InputStream;
import java.security.KeyStore;
import java.security.Principal;
import java.security.PrivateKey;
import java.security.cert.X509Certificate;
import java.util.Enumeration;
import java.util.HashMap;
import java.util.Map;
import android.view.ViewGroup;
import android.webkit.ClientCertRequest;
import android.webkit.ConsoleMessage;
import android.webkit.DownloadListener;
import android.webkit.JavascriptInterface;
import android.webkit.JsResult;
import android.webkit.SslErrorHandler;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

public class MainActivity extends Activity {
    private static final String TAG = "LibraryApp";
    private static final String TARGET_URL = Config.TARGET_URL;

    private WebView webView;
    private LinearLayout debugPanel;
    private TextView debugLog;
    private TokenStore tokenStore;
    private ReadListDB readListDB;
    private boolean hasError = false;
    private boolean offlineMode = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setLayoutParams(new ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT));

        webView = new WebView(this);
        webView.setLayoutParams(new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                0, 1.0f));

        debugPanel = createDebugPanel();
        debugPanel.setVisibility(View.GONE);

        root.addView(webView);
        root.addView(debugPanel);

        setContentView(root);

        tokenStore = new TokenStore(this);
        readListDB = new ReadListDB(this);

        setupWebView();
        setupDebug();

        Log.i(TAG, "Loading URL: " + TARGET_URL);

        // Send X-Platform header (API 21+) so server can serve mobile-optimized layout
        // Server also falls back to User-Agent detection for older API levels
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            Map<String, String> headers = new HashMap<>();
            headers.put("X-Platform", "android");
            webView.loadUrl(TARGET_URL, headers);
        } else {
            webView.loadUrl(TARGET_URL);
        }
    }

    private LinearLayout createDebugPanel() {
        LinearLayout panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setBackgroundColor(Color.parseColor("#1a1a2e"));
        panel.setLayoutParams(new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT));

        TextView header = new TextView(this);
        header.setText("=== DEBUG PANEL ===");
        header.setTextColor(Color.parseColor("#e74c3c"));
        header.setTextSize(14);
        header.setPadding(16, 16, 16, 4);
        header.setTypeface(null, android.graphics.Typeface.BOLD);
        panel.addView(header);

        ScrollView scroll = new ScrollView(this);
        scroll.setLayoutParams(new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, 400));

        debugLog = new TextView(this);
        debugLog.setTextColor(Color.WHITE);
        debugLog.setTextSize(10);
        debugLog.setPadding(16, 4, 16, 16);
        debugLog.setTypeface(android.graphics.Typeface.MONOSPACE);
        scroll.addView(debugLog);

        panel.addView(scroll);
        return panel;
    }

    private void appendDebug(String msg) {
        String text = debugLog.getText().toString();
        if (text.length() > 5000) {
            text = text.substring(text.length() - 4000);
        }
        debugLog.setText(text + "\n" + msg);
        Log.i(TAG, msg);
    }

    private void setupWebView() {
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setLoadWithOverviewMode(true);
        settings.setUseWideViewPort(true);
        settings.setAllowContentAccess(true);
        settings.setAllowFileAccess(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            settings.setSafeBrowsingEnabled(false);
        }

        // Enable remote debugging via chrome://inspect
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            WebView.setWebContentsDebuggingEnabled(true);
        }

        webView.setWebViewClient(new WebViewClient() {
            private boolean clientCertLoaded = false;

            @Override
            public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
                appendDebug("Loading: " + url);
                hasError = false;
                offlineMode = false;
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                appendDebug("Finished: " + url);
                if (!hasError || offlineMode) {
                    debugPanel.setVisibility(View.GONE);
                }
            }

            @Override
            public void onReceivedError(WebView view, WebResourceRequest request,
                                        WebResourceError error) {
                if (request != null && request.isForMainFrame()) {
                    hasError = true;
                    int code = android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M
                            ? error.getErrorCode() : -1;
                    CharSequence desc = android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M
                            ? error.getDescription() : "Unknown error";
                    String msg = "ERROR [" + code + "]: " + desc
                            + " | url: " + (request != null ? request.getUrl() : "null");
                    appendDebug(msg);
                    // Load offline page from bundled assets (works without network)
                    try {
                        java.io.InputStream is = getAssets().open("www/offline.html");
                        byte[] buffer = new byte[is.available()];
                        is.read(buffer);
                        is.close();
                        String html = new String(buffer, "UTF-8");
                        view.loadDataWithBaseURL("file:///android_asset/www/", html, "text/html", "UTF-8", null);
                        appendDebug("Loaded offline page from assets");
                        offlineMode = true;
                    } catch (java.io.IOException e) {
                        showError("HTTP Error: " + code + "\n" + desc);
                        appendDebug("Failed to load offline page: " + e.getMessage());
                    }
                }
            }

            @Override
            public void onReceivedSslError(WebView view, SslErrorHandler handler, SslError error) {
                hasError = true;
                String msg = "SSL ERROR: " + error.getPrimaryError()
                        + " (" + getSslErrorName(error.getPrimaryError()) + ")\n"
                        + "URL: " + error.getUrl()
                        + "\nCert issuer: " + (error.getCertificate() != null
                                ? error.getCertificate().getIssuedBy().getDName() : "N/A");
                appendDebug(msg);

                // In debug build — proceed anyway to diagnose further
                appendDebug(">>> SSL ERROR PROCEEDING ANYWAY (debug build) <<<");
                showError("SSL Error: " + getSslErrorName(error.getPrimaryError())
                        + "\nProceeding anyway (debug)...");
                handler.proceed();
            }

            @Override
            public void onReceivedClientCertRequest(WebView view, ClientCertRequest request) {
                appendDebug("Client certificate requested by: " + request.getHost());
                provideClientCert(request);
            }

            @Override
            public void onLoadResource(WebView view, String url) {
                if (url.startsWith("http")) {
                    appendDebug("  Resource: " + url);
                }
            }

            private void provideClientCert(ClientCertRequest request) {
                try {
                    InputStream in = getResources().openRawResource(R.raw.client_cert);
                    KeyStore ks = KeyStore.getInstance("PKCS12");
                    ks.load(in, Config.CLIENT_CERT_PASSWORD.toCharArray());
                    in.close();

                    Enumeration<String> aliases = ks.aliases();
                    if (!aliases.hasMoreElements()) {
                        appendDebug("No client cert alias found in PKCS12");
                        request.cancel();
                        return;
                    }

                    String alias = aliases.nextElement();
                    PrivateKey privateKey = (PrivateKey) ks.getKey(alias, Config.CLIENT_CERT_PASSWORD.toCharArray());
                    if (privateKey == null) {
                        appendDebug("No private key found for alias: " + alias);
                        request.cancel();
                        return;
                    }

                    java.security.cert.Certificate[] chain = ks.getCertificateChain(alias);
                    X509Certificate[] x509Chain = new X509Certificate[chain.length];
                    for (int i = 0; i < chain.length; i++) {
                        x509Chain[i] = (X509Certificate) chain[i];
                    }

                    appendDebug("Proceeding with client certificate: " + alias
                            + " (" + x509Chain[0].getSubjectDN().getName() + ")");
                    request.proceed(privateKey, x509Chain);
                } catch (Exception e) {
                    appendDebug("Client cert error: " + e.getMessage());
                    request.cancel();
                }
            }
        });

        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onConsoleMessage(ConsoleMessage msg) {
                appendDebug("JS [" + msg.messageLevel() + "]: " + msg.message()
                        + " (" + msg.sourceId() + ":" + msg.lineNumber() + ")");
                return true;
            }

            @Override
            public boolean onJsAlert(WebView view, String url, String message, JsResult result) {
                appendDebug("JS ALERT: " + message);
                result.confirm();
                return true;
            }

            @Override
            public boolean onJsConfirm(WebView view, String url, String message, JsResult result) {
                appendDebug("JS CONFIRM: " + message);
                result.confirm();
                return true;
            }
        });

        webView.addJavascriptInterface(new TokenBridge(), "AndroidTokenBridge");
        webView.addJavascriptInterface(new ReadListBridge(), "AndroidReadListDB");

        // Handle file downloads via direct HTTPS connection (trusts self-signed cert)
        webView.setDownloadListener(new DownloadListener() {
            @Override
            public void onDownloadStart(String url, String userAgent, String contentDisposition, String mimetype, long contentLength) {
                appendDebug("Download: " + url);

                // Extract filename from Content-Disposition
                final String filename;
                if (contentDisposition != null) {
                    String[] parts = contentDisposition.split("filename\\*=UTF-8''");
                    if (parts.length > 1) {
                        filename = Uri.decode(parts[1].split(";")[0].trim());
                    } else {
                        String[] parts2 = contentDisposition.split("filename=\"");
                        if (parts2.length > 1) {
                            filename = parts2[1].split("\"")[0];
                        } else {
                            filename = "book.zip";
                        }
                    }
                } else {
                    filename = "book.zip";
                }

                final String downloadUrl = url;
                new Thread(new Runnable() {
                    @Override
                    public void run() {
                        downloadFile(downloadUrl, filename);
                    }
                }).start();
            }
        });
    }

    private void downloadFile(String urlStr, String filename) {
        java.io.BufferedInputStream bis = null;
        java.io.FileOutputStream fos = null;
        try {
            // Create SSL context that trusts all certificates (self-signed dev cert)
            javax.net.ssl.TrustManager[] trustAll = new javax.net.ssl.TrustManager[]{
                new javax.net.ssl.X509TrustManager() {
                    public java.security.cert.X509Certificate[] getAcceptedIssuers() { return new java.security.cert.X509Certificate[0]; }
                    public void checkClientTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                    public void checkServerTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                }
            };
            javax.net.ssl.SSLContext sc = javax.net.ssl.SSLContext.getInstance("TLS");
            sc.init(null, trustAll, new java.security.SecureRandom());

            java.net.URL url = new java.net.URL(urlStr);
            javax.net.ssl.HttpsURLConnection conn = (javax.net.ssl.HttpsURLConnection) url.openConnection();
            conn.setSSLSocketFactory(sc.getSocketFactory());
            conn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
            });
            conn.setRequestMethod("GET");
            conn.setConnectTimeout(15000);
            conn.setReadTimeout(30000);
            conn.connect();

            int responseCode = conn.getResponseCode();
            appendDebug("Download response: " + responseCode);
            if (responseCode != 200) {
                showError("Download failed: HTTP " + responseCode);
                return;
            }

            // Save to Downloads directory
            java.io.File downloadsDir = android.os.Environment.getExternalStoragePublicDirectory(
                    android.os.Environment.DIRECTORY_DOWNLOADS);
            java.io.File outFile = new java.io.File(downloadsDir, filename);

            // Ensure unique filename
            int counter = 1;
            while (outFile.exists()) {
                String name = filename.substring(0, filename.lastIndexOf('.'));
                String ext = filename.substring(filename.lastIndexOf('.'));
                outFile = new java.io.File(downloadsDir, name + " (" + counter + ")" + ext);
                counter++;
            }

            bis = new java.io.BufferedInputStream(conn.getInputStream());
            fos = new java.io.FileOutputStream(outFile);
            byte[] buffer = new byte[8192];
            int bytesRead;
            long total = 0;
            while ((bytesRead = bis.read(buffer)) != -1) {
                fos.write(buffer, 0, bytesRead);
                total += bytesRead;
            }
            fos.flush();

            appendDebug("Download saved: " + outFile.getAbsolutePath() + " (" + total + " bytes)");

            // Notify via Toast on UI thread
            final String msg = "Скачано: " + outFile.getName();
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    Toast.makeText(MainActivity.this, msg, Toast.LENGTH_LONG).show();
                }
            });
        } catch (Exception e) {
            appendDebug("Download error: " + e.getMessage());
            showError("Download error: " + e.getMessage());
        } finally {
            try { if (bis != null) bis.close(); } catch (Exception e) {}
            try { if (fos != null) fos.close(); } catch (Exception e) {}
        }
    }

    private class TokenBridge {
        @JavascriptInterface
        public void storeRefreshToken(String token) {
            Log.i(TAG, "Storing refresh token via JS bridge");
            tokenStore.storeRefreshToken(token);
        }

        @JavascriptInterface
        public String getRefreshToken() {
            String token = tokenStore.getRefreshToken();
            Log.i(TAG, "Getting refresh token via JS bridge: " + (token != null ? "found" : "null"));
            return token;
        }

        @JavascriptInterface
        public void clearRefreshToken() {
            Log.i(TAG, "Clearing refresh token via JS bridge");
            tokenStore.clearRefreshToken();
        }
    }

    private class ReadListBridge {
        @JavascriptInterface
        public void replaceAll(String jsonArray) {
            readListDB.replaceAll(jsonArray);
        }

        @JavascriptInterface
        public String queryAll(String listname, String bookname, String author, String status) {
            return readListDB.queryAll(listname, bookname, author, status);
        }

        @JavascriptInterface
        public void upsertItem(String jsonString) {
            readListDB.upsertItem(jsonString);
        }

        @JavascriptInterface
        public void deleteItem(String id) {
            readListDB.deleteItem(id);
        }

        @JavascriptInterface
        public void clearAll() {
            readListDB.clearAll();
        }

        @JavascriptInterface
        public String getPendingQueue() {
            return readListDB.getPendingQueue();
        }

        @JavascriptInterface
        public int getPendingCount() {
            return readListDB.getPendingCount();
        }

        @JavascriptInterface
        public void enqueue(String operation, String itemId, String body) {
            readListDB.enqueue(operation, itemId, body);
        }

        @JavascriptInterface
        public void enqueueDelete(String itemId) {
            readListDB.enqueueDelete(itemId);
        }

        @JavascriptInterface
        public void clearQueue() {
            readListDB.clearQueue();
        }

        @JavascriptInterface
        public void dequeue(int queueId) {
            readListDB.dequeue(queueId);
        }
    }

    private void setupDebug() {
        // Touch 5 times on debug panel header to show/hide
        debugPanel.setOnClickListener(new View.OnClickListener() {
            int tapCount = 0;
            long lastTap = 0;
            @Override
            public void onClick(View v) {
                long now = System.currentTimeMillis();
                if (now - lastTap > 2000) tapCount = 0;
                lastTap = now;
                tapCount++;
                if (tapCount >= 3) {
                    tapCount = 0;
                    debugPanel.setVisibility(
                            debugPanel.getVisibility() == View.GONE ? View.VISIBLE : View.GONE);
                }
            }
        });
    }

    private void showError(String msg) {
        debugPanel.setVisibility(View.VISIBLE);
        Toast.makeText(this, msg, Toast.LENGTH_LONG).show();
    }

    private String getSslErrorName(int code) {
        switch (code) {
            case SslError.SSL_NOTYETVALID:    return "SSL_NOTYETVALID";
            case SslError.SSL_EXPIRED:        return "SSL_EXPIRED";
            case SslError.SSL_IDMISMATCH:     return "SSL_IDMISMATCH";
            case SslError.SSL_UNTRUSTED:      return "SSL_UNTRUSTED";
            case SslError.SSL_DATE_INVALID:   return "SSL_DATE_INVALID";
            case SslError.SSL_INVALID:        return "SSL_INVALID";
            default:                          return "UNKNOWN(" + code + ")";
        }
    }

    @Override
    public void onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }
}
