package app.library.twa;

import android.app.Activity;
import android.app.AlertDialog;
import android.app.DownloadManager;
import android.content.Context;
import android.content.ActivityNotFoundException;
import android.content.ContentResolver;
import android.content.DialogInterface;
import android.content.Intent;
import android.database.Cursor;
import android.graphics.Color;
import android.provider.DocumentsContract;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Build;
import android.os.Handler;
import android.os.Bundle;
import android.os.Environment;
import android.util.Log;
import android.view.Gravity;
import android.view.View;
import androidx.core.content.FileProvider;
import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
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
import android.webkit.JsResult;
import android.webkit.SslErrorHandler;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
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
    private ValueCallback<Uri[]> mUploadMessage;
    private static final int FILECHOOSER_RESULTCODE = 1001;
    private static final int FOLDER_PICKER_RESULTCODE = 1002;
    private String pendingFolderAuthToken;
    private boolean hasError = false;
    private boolean offlineMode = false;
    private boolean forceNetworkRefresh = false;
    private TokenBridge tokenBridge;
    private Handler startupTimeoutHandler = new Handler();
    private Runnable startupTimeoutRunnable = new Runnable() {
        @Override
        public void run() {
            if (offlineMode) return;
            appendDebug("Startup: 5s timeout reached, checking page status");
            webView.evaluateJavascript(
                "(function(){var b=document.body;if(!b)return false;" +
                "var hasContent=b.querySelector('.tabs,.tab-content,.container,.loading,.error,.empty,.tree-view,#authorsTree,#booksTableContainer,#readlistTableContainer');" +
                "return !!hasContent;})()",
                new android.webkit.ValueCallback<String>() {
                    @Override
                    public void onReceiveValue(String value) {
                        if (value != null && value.contains("true")) {
                            appendDebug("Page has content, keeping current page");
                            startupTimeoutHandler.removeCallbacks(startupTimeoutRunnable);
                        } else {
                            appendDebug("Page appears blank, loading offline page");
                            loadOfflinePage();
                        }
                    }
                }
            );
        }
    };

    // ── Static file asset injection (matches Go server's mobile injection) ──
    private static final String MOBILE_CSS_TAG =
        "<link rel=\"stylesheet\" href=\"/static/css/mobile.css\">";
    private static final String ANDROID_BODY =
        "<body class=\"android\">";
    private static final String MOBILE_TOP_BAR_INDEX =
        "<div class=\"mobile-top-bar\">\n" +
        "    <a href=\"/admin\" class=\"mobile-admin-btn\" title=\"Администрирование\">А</a>\n" +
        "    <span class=\"mobile-top-spacer\"></span>\n" +
        "    <button class=\"mobile-user-btn\" id=\"mobileUserBtn\" title=\"Пользователь\">☰</button>\n" +
        "</div>";
    private static final String MOBILE_TOP_BAR_ADMIN =
        "<div class=\"mobile-top-bar\">\n" +
        "    <a href=\"/\" class=\"mobile-back-btn\" title=\"Назад к библиотеке\">←</a>\n" +
        "    <span class=\"mobile-top-title\">Админ</span>\n" +
        "    <span class=\"mobile-top-spacer\"></span>\n" +
        "    <button class=\"mobile-user-btn\" id=\"mobileUserBtn\" title=\"Пользователь\">☰</button>\n" +
        "</div>";
    private static final String ANDROID_JS =
        "<script>\n" +
        "(function(){\n" +
        "var a=document.body.classList.contains('android');\n" +
        "if(!a)return;\n" +
        "var q=function(s){return document.querySelector(s)};\n" +
        "var qa=function(s){return document.querySelectorAll(s)};\n" +
        "function updateMobileUser(){\n" +
        "var btn=document.getElementById('mobileUserBtn');\n" +
        "if(!btn)return;\n" +
        "try{\n" +
        "var stored=localStorage.getItem('auth_user');\n" +
        "if(stored){\n" +
        "var user=JSON.parse(stored);\n" +
        "if(user&&user.username){\n" +
        "btn.textContent=user.username.charAt(0).toUpperCase();\n" +
        "btn.classList.add('logged-in');\n" +
        "return;\n" +
        "}\n" +
        "}\n" +
        "}catch(e){}\n" +
        "btn.textContent='\\u2630';\n" +
        "btn.classList.remove('logged-in');\n" +
        "}\n" +
        "window.updateMobileUser=updateMobileUser;\n" +
        "updateMobileUser();\n" +
        "setInterval(updateMobileUser,1000);\n" +
        "document.getElementById('mobileUserBtn')?.addEventListener('click',function(){\n" +
        "if(localStorage.getItem('auth_user')){\n" +
        "if(confirm('Вы хотите завершить сессию пользователя?')){\n" +
        "localStorage.removeItem('auth_token');\n" +
        "localStorage.removeItem('auth_user');\n" +
        "window.location.reload();\n" +
        "}\n" +
        "}else{\n" +
        "var lb=document.getElementById('loginBtn');\n" +
        "if(lb)lb.click();\n" +
        "}\n" +
        "});\n" +
        "['books','tab-books'].forEach(function(tabId){\n" +
        "var el=document.getElementById(tabId);\n" +
        "if(!el)return;\n" +
        "var obs=new MutationObserver(function(){\n" +
        "if(el.classList.contains('active'))updateMobileUser();\n" +
        "});\n" +
        "obs.observe(el,{attributes:true,attributeFilter:['class']});\n" +
        "});\n" +
        "if(window.AndroidHttpProxy&&typeof window.AndroidHttpProxy.httpRequest==='function'){\n" +
        "var _origFetch=window.fetch;\n" +
        "window.fetch=function(url,opts){\n" +
        "var u=typeof url==='string'?url:url.url;\n" +
        "if(u&&u.indexOf('/api/')!==-1){\n" +
        "return new Promise(function(resolve,reject){\n" +
        "try{\n" +
        "var m=(opts&&opts.method||'GET').toUpperCase();\n" +
        "var h=opts&&opts.headers?JSON.stringify(opts.headers):\n" +
        "JSON.stringify({'Content-Type':'application/json'});\n" +
        "var b='';\n" +
        "if(opts&&opts.body){b=typeof opts.body==='string'?opts.body:JSON.stringify(opts.body);}\n" +
        "var full=u;\n" +
        "if(u.indexOf('http')!==0){full=location.origin+u;}\n" +
        "var r=window.AndroidHttpProxy.httpRequest(m,full,h,b);\n" +
        "var data=JSON.parse(r);\n" +
        "var resp=new Response(data.body,{\n" +
        "status:data.status,\n" +
        "statusText:data.status>=200&&data.status<300?'OK':'Error',\n" +
        "headers:data.headers||{}\n" +
        "});\n" +
        "resolve(resp);\n" +
        "}catch(e){\n" +
        "resolve(new Response(JSON.stringify({error:e.message}),{status:502}));\n" +
        "}\n" +
        "});\n" +
        "}\n" +
        "return _origFetch.apply(this,arguments);\n" +
        "};\n" +
        "}\n" +
        "})();\n" +
        "</script>";

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

        Log.i(TAG, "Loading main page from assets (no network needed)");

        // Set window background to match app theme (prevents white flash while WebView loads)
        getWindow().setBackgroundDrawable(new android.graphics.drawable.ColorDrawable(
            android.graphics.Color.parseColor("#f5f5f5")));

        // Load main page directly from assets — no network request at all
        loadMainPageFromAssets();

        // Start 5-second startup timeout — if page doesn't start loading, switch to offline
        startupTimeoutHandler.postDelayed(startupTimeoutRunnable, 3000);

        // Schedule APK update check in background with delay (no WebView bridge contention)
        // Runs once, short timeout, no retry on failure
        startupTimeoutHandler.postDelayed(new Runnable() {
            @Override
            public void run() {
                if (tokenBridge != null) {
                    new Thread(new Runnable() {
                        @Override
                        public void run() {
                            tokenBridge.triggerUpdateCheck();
                        }
                    }).start();
                }
            }
        }, 8000);
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

            // Intercept download URLs to prevent WebView from navigating to binary content
            @Override
            @SuppressWarnings("deprecation")
            public boolean shouldOverrideUrlLoading(WebView view, String url) {
                if (url != null && url.contains("/download")) {
                    startDownload(url);
                    return true;
                }
                return false;
            }

            @Override
            @android.annotation.TargetApi(Build.VERSION_CODES.N)
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url != null && url.contains("/download")) {
                    startDownload(url);
                    return true;
                }
                return false;
            }

            @Override
            public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
                appendDebug("Loading: " + url);
                hasError = false;
                offlineMode = false;
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                appendDebug("Finished: " + url);
                forceNetworkRefresh = false;
                webView.evaluateJavascript(
                    "(function(){var b=document.body;if(!b)return false;" +
                    "var hasContent=b.querySelector('.tabs,.tab-content,.container,.loading,.error,.empty,.tree-view,#authorsTree,#booksTableContainer,#readlistTableContainer');" +
                    "return !!hasContent;})()",
                    new android.webkit.ValueCallback<String>() {
                        @Override
                        public void onReceiveValue(String value) {
                            if (value != null && value.contains("true")) {
                                startupTimeoutHandler.removeCallbacks(startupTimeoutRunnable);
                                if (!hasError || offlineMode) {
                                    debugPanel.setVisibility(View.GONE);
                                }
                            } else {
                                appendDebug("Page appears blank after load, watchdog will handle");
                            }
                        }
                    }
                );
            }

            @Override
            public void onReceivedError(WebView view, WebResourceRequest request,
                                        WebResourceError error) {
                if (request != null && request.isForMainFrame()) {
                    hasError = true;
                    startupTimeoutHandler.removeCallbacks(startupTimeoutRunnable);
                    int code = android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M
                            ? error.getErrorCode() : -1;
                    CharSequence desc = android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M
                            ? error.getDescription() : "Unknown error";
                    String msg = "ERROR [" + code + "]: " + desc
                            + " | url: " + (request != null ? request.getUrl() : "null");
                    appendDebug(msg);
                    loadOfflinePage();
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

            @Override
            public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                String path = request.getUrl().getPath();

                // When forceNetworkRefresh is true (user just logged in), bypass asset cache
                if (forceNetworkRefresh) {
                    return null;
                }

                // Serve main pages from assets
                if ("/".equals(path) || path.isEmpty()) {
                    WebResourceResponse res = serveIndexFromAssets();
                    if (res != null) return res;
                }
                if ("/admin".equals(path)) {
                    WebResourceResponse res = serveAdminFromAssets();
                    if (res != null) return res;
                }
                // Serve static files from assets
                if (path != null && path.startsWith("/static/")) {
                    String assetPath = "www" + path;
                    String mime = getMimeType(path);
                    if (mime == null) return null;
                    WebResourceResponse res = serveFromAssets(assetPath, mime);
                    if (res != null) return res;
                }

                // Serve service worker from assets
                if ("/service-worker.js".equals(path)) {
                    WebResourceResponse res = serveFromAssets("www/service-worker.js", "application/javascript");
                    if (res != null) return res;
                }

                // Serve favicon from assets
                if ("/favicon.ico".equals(path) || "/favicon.svg".equals(path)) {
                    String assetPath = "www" + path;
                    String mime = path.endsWith(".svg") ? "image/svg+xml" : "image/x-icon";
                    WebResourceResponse res = serveFromAssets(assetPath, mime);
                    if (res != null) return res;
                }

                return null; // default: network
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
            public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> filePathCallback, FileChooserParams fileChooserParams) {
                if (mUploadMessage != null) {
                    mUploadMessage.onReceiveValue(null);
                    mUploadMessage = null;
                }
                mUploadMessage = filePathCallback;

                Intent intent = fileChooserParams.createIntent();
                try {
                    startActivityForResult(intent, FILECHOOSER_RESULTCODE);
                } catch (ActivityNotFoundException e) {
                    mUploadMessage = null;
                    return false;
                }
                return true;
            }

        });

        tokenBridge = new TokenBridge();
        webView.addJavascriptInterface(tokenBridge, "AndroidTokenBridge");
        webView.addJavascriptInterface(new ReadListBridge(), "AndroidReadListDB");
        webView.addJavascriptInterface(new FileImportBridge(), "AndroidFileImport");
        webView.addJavascriptInterface(new HttpProxyBridge(), "AndroidHttpProxy");

        // Handle file downloads via direct HTTPS connection (trusts self-signed cert)
        webView.setDownloadListener(new DownloadListener() {
            @Override
            public void onDownloadStart(String url, String userAgent, String contentDisposition, String mimetype, long contentLength) {
                try {
                    appendDebug("Download: " + url);
                    final String filename = parseFilename(contentDisposition);
                    final String downloadUrl = url;
                    new Thread(new Runnable() {
                        @Override
                        public void run() {
                            downloadFile(downloadUrl, filename);
                        }
                    }).start();
                } catch (Exception e) {
                    appendDebug("Download onDownloadStart error: " + e.getMessage());
                }
            }
        });
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == FILECHOOSER_RESULTCODE) {
            if (mUploadMessage == null) return;
            Uri[] results = null;
            if (resultCode == Activity.RESULT_OK) {
                if (data != null) {
                    String dataString = data.getDataString();
                    if (dataString != null) {
                        results = new Uri[]{Uri.parse(dataString)};
                    }
                }
            }
            mUploadMessage.onReceiveValue(results);
            mUploadMessage = null;
        } else if (requestCode == FOLDER_PICKER_RESULTCODE) {
            if (resultCode == Activity.RESULT_OK && data != null) {
                final Uri treeUri = data.getData();
                final int takeFlags = data.getFlags() & Intent.FLAG_GRANT_READ_URI_PERMISSION;
                try {
                    getContentResolver().takePersistableUriPermission(treeUri, takeFlags);
                } catch (SecurityException e) {
                    appendDebug("No persistable permission: " + e.getMessage());
                }
                new Thread(new Runnable() {
                    @Override
                    public void run() {
                        importFolderFromTreeUri(treeUri);
                    }
                }).start();
            } else {
                callJsCallback("{\"error\":\"Folder selection cancelled\"}");
            }
        } else {
            super.onActivityResult(requestCode, resultCode, data);
        }
    }

    private void startDownload(final String urlStr) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                appendDebug("startDownload: " + urlStr);
            }
        });
        new Thread(new Runnable() {
            @Override
            public void run() {
                java.io.BufferedInputStream bis = null;
                java.io.FileOutputStream fos = null;
                try {
                    java.net.URL url = new java.net.URL(urlStr);
                    java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();
                    if (urlStr.startsWith("https")) {
                        // Load client certificate for mTLS
                        javax.net.ssl.KeyManager[] keyManagers = null;
                        try {
                            java.io.InputStream certIn = getResources().openRawResource(R.raw.client_cert);
                            java.security.KeyStore clientKs = java.security.KeyStore.getInstance("PKCS12");
                            clientKs.load(certIn, Config.CLIENT_CERT_PASSWORD.toCharArray());
                            certIn.close();
                            javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
                                    javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
                            kmf.init(clientKs, Config.CLIENT_CERT_PASSWORD.toCharArray());
                            keyManagers = kmf.getKeyManagers();
                        } catch (Exception e) {
                            appendDebug("Client cert load failed: " + e.getMessage());
                        }

                        javax.net.ssl.TrustManager[] trustAll = new javax.net.ssl.TrustManager[]{
                            new javax.net.ssl.X509TrustManager() {
                                public java.security.cert.X509Certificate[] getAcceptedIssuers() { return new java.security.cert.X509Certificate[0]; }
                                public void checkClientTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                                public void checkServerTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                            }
                        };
                        javax.net.ssl.SSLContext sc = javax.net.ssl.SSLContext.getInstance("TLS");
                        sc.init(keyManagers, trustAll, new java.security.SecureRandom());
                        javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                        httpsConn.setSSLSocketFactory(sc.getSocketFactory());
                        httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                            public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                        });
                    }
                    conn.setRequestMethod("GET");
                    conn.setConnectTimeout(15000);
                    conn.setReadTimeout(30000);
                    conn.setInstanceFollowRedirects(true);

                    // Forward auth cookies (sanitized — Android CookieManager may insert \n)
                    String cookies = android.webkit.CookieManager.getInstance().getCookie(urlStr);
                    if (cookies != null && !cookies.isEmpty()) {
                        conn.setRequestProperty("Cookie", cookies.replaceAll("[\\r\\n]+", "; "));
                    }

                    conn.connect();

                    final int responseCode = conn.getResponseCode();
                    if (responseCode != 200) {
                        StringBuilder errBody = new StringBuilder();
                        try {
                            java.io.InputStream errStream = conn.getErrorStream();
                            if (errStream != null) {
                                java.io.BufferedReader br = new java.io.BufferedReader(new java.io.InputStreamReader(errStream));
                                String line;
                                while ((line = br.readLine()) != null) errBody.append(line);
                                br.close();
                            }
                        } catch (Exception ignored) {}
                        final String errDetail = errBody.toString();
                        appendDebug("Download HTTP " + responseCode + " body: " + errDetail);
                        showErrorUi("Download failed: HTTP " + responseCode);
                        return;
                    }

                    String contentDisposition = conn.getHeaderField("Content-Disposition");
                    String filename = parseFilename(contentDisposition);

                    java.io.File downloadsDir = android.os.Environment.getExternalStoragePublicDirectory(
                            android.os.Environment.DIRECTORY_DOWNLOADS);
                    java.io.File outFile = resolveUniqueFile(downloadsDir, filename);

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

                    final String msg = "Скачано: " + outFile.getName();
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            Toast.makeText(MainActivity.this, msg, Toast.LENGTH_LONG).show();
                        }
                    });
                } catch (final Exception e) {
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            appendDebug("Download error: " + e.getMessage());
                            showError("Download error: " + e.getMessage());
                        }
                    });
                } finally {
                    try { if (bis != null) bis.close(); } catch (Exception e) {}
                    try { if (fos != null) fos.close(); } catch (Exception e) {}
                }
            }
        }).start();
    }

    private String parseFilename(String contentDisposition) {
        if (contentDisposition == null) return "book.zip";
        // Try filename*=UTF-8'' format first
        String[] parts = contentDisposition.split("filename\\*=UTF-8''");
        if (parts.length > 1) {
            String name = Uri.decode(parts[1].split(";")[0].trim());
            if (name != null && !name.isEmpty()) return name;
        }
        // Fallback to filename="..."
        String[] parts2 = contentDisposition.split("filename=\"");
        if (parts2.length > 1) {
            String name = parts2[1].split("\"")[0];
            if (name != null && !name.isEmpty()) return name;
        }
        return "book.zip";
    }

    private java.io.File resolveUniqueFile(java.io.File dir, String filename) {
        java.io.File f = new java.io.File(dir, filename);
        if (!f.exists()) return f;
        int dot = filename.lastIndexOf('.');
        String base = (dot > 0) ? filename.substring(0, dot) : filename;
        String ext = (dot > 0) ? filename.substring(dot) : "";
        int counter = 1;
        while (f.exists()) {
            f = new java.io.File(dir, base + " (" + counter + ")" + ext);
            counter++;
        }
        return f;
    }

    private void showErrorUi(final String msg) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                showError(msg);
            }
        });
    }

    private void downloadFile(String urlStr, String filename) {
        java.io.BufferedInputStream bis = null;
        java.io.FileOutputStream fos = null;
        try {
            java.net.URL url = new java.net.URL(urlStr);
            java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();
            if (urlStr.startsWith("https")) {
                javax.net.ssl.KeyManager[] keyManagers = null;
                try {
                    java.io.InputStream certIn = getResources().openRawResource(R.raw.client_cert);
                    java.security.KeyStore clientKs = java.security.KeyStore.getInstance("PKCS12");
                    clientKs.load(certIn, Config.CLIENT_CERT_PASSWORD.toCharArray());
                    certIn.close();
                    javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
                            javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
                    kmf.init(clientKs, Config.CLIENT_CERT_PASSWORD.toCharArray());
                    keyManagers = kmf.getKeyManagers();
                } catch (Exception e) {
                    appendDebug("Client cert load failed: " + e.getMessage());
                }

                javax.net.ssl.TrustManager[] trustAll = new javax.net.ssl.TrustManager[]{
                    new javax.net.ssl.X509TrustManager() {
                        public java.security.cert.X509Certificate[] getAcceptedIssuers() { return new java.security.cert.X509Certificate[0]; }
                        public void checkClientTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                        public void checkServerTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                    }
                };
                javax.net.ssl.SSLContext sc = javax.net.ssl.SSLContext.getInstance("TLS");
                sc.init(keyManagers, trustAll, new java.security.SecureRandom());
                javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                httpsConn.setSSLSocketFactory(sc.getSocketFactory());
                httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                    public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                });
            }
            conn.setRequestMethod("GET");
            conn.setConnectTimeout(15000);
            conn.setReadTimeout(30000);
            conn.setInstanceFollowRedirects(true);

            // Forward auth cookies (sanitized — Android CookieManager may insert \n)
            String cookies = android.webkit.CookieManager.getInstance().getCookie(urlStr);
            if (cookies != null && !cookies.isEmpty()) {
                conn.setRequestProperty("Cookie", cookies.replaceAll("[\\r\\n]+", "; "));
            }

            conn.connect();

            int responseCode = conn.getResponseCode();
            if (responseCode != 200) {
                StringBuilder errBody = new StringBuilder();
                try {
                    java.io.InputStream errStream = conn.getErrorStream();
                    if (errStream != null) {
                        java.io.BufferedReader br = new java.io.BufferedReader(new java.io.InputStreamReader(errStream));
                        String line;
                        while ((line = br.readLine()) != null) errBody.append(line);
                        br.close();
                    }
                } catch (Exception ignored) {}
                appendDebug("downloadFile HTTP " + responseCode + " body: " + errBody.toString());
                showErrorUi("Download failed: HTTP " + responseCode);
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

            // Notify via Toast on UI thread
            final long savedTotal = total;
            final String savedPath = outFile.getAbsolutePath();
            final String msg = "Скачано: " + outFile.getName();
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    appendDebug("Download saved: " + savedPath + " (" + savedTotal + " bytes)");
                    Toast.makeText(MainActivity.this, msg, Toast.LENGTH_LONG).show();
                }
            });
        } catch (final Exception e) {
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    appendDebug("Download error: " + e.getMessage());
                    showError("Download error: " + e.getMessage());
                }
            });
        } finally {
            try { if (bis != null) bis.close(); } catch (Exception e) {}
            try { if (fos != null) fos.close(); } catch (Exception e) {}
        }
    }

    // ── Folder Import (Android bridge) ──────────────────────────────
    private void importFolderFromTreeUri(Uri treeUri) {
        try {
            final java.util.ArrayList<String> fileUris = new java.util.ArrayList<String>();
            final java.util.ArrayList<String> fileNames = new java.util.ArrayList<String>();
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                enumerateFiles(treeUri, treeUri, fileUris, fileNames);
            }

            if (fileUris.isEmpty()) {
                callJsCallback("{\"error\":\"В выбранной папке не найдены книги\"}");
                return;
            }

            uploadFilesToServer(fileUris, fileNames);
        } catch (Exception e) {
            appendDebug("Folder import error: " + e.getMessage());
            String err = e.getMessage();
            if (err == null) err = "Unknown error";
            callJsCallback("{\"error\":\"" + err.replace("\\", "\\\\").replace("\"", "\\\"") + "\"}");
        }
    }

    @android.annotation.TargetApi(21)
    private void enumerateFiles(Uri treeUri, Uri dirUri, java.util.ArrayList<String> fileUris, java.util.ArrayList<String> fileNames) {
        String dirDocId = DocumentsContract.getTreeDocumentId(dirUri);
        Uri childrenUri = DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, dirDocId);
        Cursor cursor = null;
        try {
            cursor = getContentResolver().query(childrenUri,
                    new String[]{
                        DocumentsContract.Document.COLUMN_DOCUMENT_ID,
                        DocumentsContract.Document.COLUMN_DISPLAY_NAME,
                        DocumentsContract.Document.COLUMN_MIME_TYPE
                    }, null, null, null);
            if (cursor != null) {
                while (cursor.moveToNext()) {
                    String docId = cursor.getString(0);
                    String name = cursor.getString(1);
                    String mimeType = cursor.getString(2);
                    if (DocumentsContract.Document.MIME_TYPE_DIR.equals(mimeType)) {
                        Uri subDirUri = DocumentsContract.buildDocumentUriUsingTree(treeUri, docId);
                        enumerateFiles(treeUri, subDirUri, fileUris, fileNames);
                    } else {
                        String lower = name.toLowerCase();
                        if (lower.endsWith(".fb2") || lower.endsWith(".epub") ||
                            lower.endsWith(".pdf") || lower.endsWith(".doc") ||
                            lower.endsWith(".docx") || lower.endsWith(".zip") ||
                            lower.endsWith(".fb2.zip")) {
                            Uri fileUri = DocumentsContract.buildDocumentUriUsingTree(treeUri, docId);
                            fileUris.add(fileUri.toString());
                            fileNames.add(name);
                        }
                    }
                }
            }
        } catch (Exception e) {
            appendDebug("Enumerate error: " + e.getMessage());
        } finally {
            if (cursor != null) cursor.close();
        }
    }

    private void uploadFilesToServer(java.util.ArrayList<String> fileUris, java.util.ArrayList<String> fileNames) {
        java.io.OutputStream os = null;
        java.io.InputStream is = null;
        try {
            String boundary = "Boundary-" + System.currentTimeMillis();
            String targetUrl = Config.TARGET_URL;
            if (!targetUrl.endsWith("/")) targetUrl += "/";
            targetUrl += "api/v1/import/upload";

            java.net.URL url = new java.net.URL(targetUrl);
            javax.net.ssl.HttpsURLConnection conn = (javax.net.ssl.HttpsURLConnection) url.openConnection();
            conn.setSSLSocketFactory(createTrustAllSslSocketFactory());
            conn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
            });
            conn.setRequestMethod("POST");
            conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=" + boundary);
            if (pendingFolderAuthToken != null) {
                conn.setRequestProperty("Authorization", "Bearer " + pendingFolderAuthToken);
            }
            conn.setDoOutput(true);
            conn.setConnectTimeout(30000);
            conn.setReadTimeout(120000);

            os = conn.getOutputStream();
            byte[] buffer = new byte[8192];
            for (int i = 0; i < fileUris.size(); i++) {
                os.write(("--" + boundary + "\r\n").getBytes());
                String disposition = "Content-Disposition: form-data; name=\"files\"; filename=\"" + fileNames.get(i) + "\"\r\n";
                os.write(disposition.getBytes("UTF-8"));
                os.write("Content-Type: application/octet-stream\r\n\r\n".getBytes());

                is = getContentResolver().openInputStream(Uri.parse(fileUris.get(i)));
                int n;
                while ((n = is.read(buffer)) != -1) {
                    os.write(buffer, 0, n);
                }
                is.close();
                is = null;
                os.write("\r\n".getBytes());
            }
            os.write(("--" + boundary + "--\r\n").getBytes());
            os.close();
            os = null;

            int responseCode = conn.getResponseCode();
            java.io.InputStream responseStream = (responseCode >= 200 && responseCode < 300)
                ? conn.getInputStream() : conn.getErrorStream();
            java.io.BufferedReader reader = new java.io.BufferedReader(
                new java.io.InputStreamReader(responseStream, "UTF-8"));
            StringBuilder response = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) {
                response.append(line);
            }
            reader.close();

            if (responseCode >= 200 && responseCode < 300) {
                appendDebug("Folder import upload OK: " + responseCode + " files=" + fileUris.size());
                callJsCallback(response.toString());
            } else {
                appendDebug("Folder import upload failed: " + responseCode + " " + response.toString());
                callJsCallback("{\"error\":\"HTTP " + responseCode + ": " + response.toString().replace("\\", "\\\\").replace("\"", "\\\"") + "\"}");
            }
        } catch (Exception e) {
            appendDebug("Upload error: " + e.getMessage());
            String err = e.getMessage();
            if (err == null) err = "Upload failed";
            callJsCallback("{\"error\":\"" + err.replace("\\", "\\\\").replace("\"", "\\\"") + "\"}");
        } finally {
            try { if (os != null) os.close(); } catch (Exception e) {}
            try { if (is != null) is.close(); } catch (Exception e) {}
            pendingFolderAuthToken = null;
        }
    }

    private void callJsCallback(final String jsonResult) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                String js = "if(window._folderImportCallback)_folderImportCallback('" +
                    jsonResult.replace("\\", "\\\\").replace("'", "\\'").replace("\n", "\\n") + "')";
                webView.evaluateJavascript(js, null);
            }
        });
    }

    private void callAuthBridgeCallback(final int code, final String body) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                String escaped = body.replace("\\", "\\\\").replace("'", "\\'").replace("\n", "\\n");
                String js = "if(window._authBridgeCallback)_authBridgeCallback(" + code + ",'" + escaped + "')";
                webView.evaluateJavascript(js, null);
            }
        });
    }

    private javax.net.ssl.SSLSocketFactory createTrustAllSslSocketFactory() throws Exception {
        javax.net.ssl.KeyManager[] keyManagers = null;
        try {
            java.io.InputStream certIn = getResources().openRawResource(R.raw.client_cert);
            java.security.KeyStore clientKs = java.security.KeyStore.getInstance("PKCS12");
            clientKs.load(certIn, Config.CLIENT_CERT_PASSWORD.toCharArray());
            certIn.close();
            javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
                    javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
            kmf.init(clientKs, Config.CLIENT_CERT_PASSWORD.toCharArray());
            keyManagers = kmf.getKeyManagers();
        } catch (Exception e) {
            appendDebug("Client cert load failed: " + e.getMessage());
        }
        javax.net.ssl.TrustManager[] trustAll = new javax.net.ssl.TrustManager[]{
            new javax.net.ssl.X509TrustManager() {
                public java.security.cert.X509Certificate[] getAcceptedIssuers() { return new java.security.cert.X509Certificate[0]; }
                public void checkClientTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
                public void checkServerTrusted(java.security.cert.X509Certificate[] certs, String authType) {}
            }
        };
        javax.net.ssl.SSLContext sc = javax.net.ssl.SSLContext.getInstance("TLS");
        sc.init(keyManagers, trustAll, new java.security.SecureRandom());
        return sc.getSocketFactory();
    }

    private class FileImportBridge {
        @JavascriptInterface
        public void pickAndImportFolder(final String authToken) {
            pendingFolderAuthToken = authToken;
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
                    intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                    startActivityForResult(intent, FOLDER_PICKER_RESULTCODE);
                }
            });
        }
    }

    private class TokenBridge {
        private String authToken = "";

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

        @JavascriptInterface
        public void setForceNetworkRefresh(boolean force) {
            Log.i(TAG, "Force network refresh set to: " + force);
            forceNetworkRefresh = force;
        }

        @JavascriptInterface
        public void setAuthToken(String token) {
            Log.i(TAG, "Setting auth token via JS bridge");
            this.authToken = token;
        }

        @JavascriptInterface
        public String getAppVersion() {
            return Config.APK_VERSION_NAME;
        }

        public void triggerUpdateCheck() {
            Log.i(TAG, "Triggered update check from Java (background, no WebView bridge)");
            checkForUpdateInternal();
        }

        @JavascriptInterface
        public void checkForUpdate() {
            Log.i(TAG, "Manual checkForUpdate called from JS");
            checkForUpdateInternal();
        }

        private void checkForUpdateInternal() {
            Log.i(TAG, "Checking for APK update in background thread");
            new Thread(new Runnable() {
                @Override
                public void run() {
                    try {
                        String targetUrl = Config.TARGET_URL;
                        java.net.URL url = new java.net.URL(targetUrl + "api/v1/apk/version");
                        java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();

                        if (url.getProtocol().equals("https")) {
                            javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                            javax.net.ssl.SSLSocketFactory sf = createEmbeddedCertSSLSocketFactory();
                            if (sf != null) {
                                httpsConn.setSSLSocketFactory(sf);
                            }
                            httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                                public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                            });
                        }
                        conn.setRequestMethod("GET");
                        conn.setRequestProperty("Authorization", "Bearer " + authToken);
                        conn.setConnectTimeout(2000);
                        conn.setReadTimeout(3000);

                        int responseCode = conn.getResponseCode();
                        if (responseCode != 200) return;

                        java.io.BufferedReader reader = new java.io.BufferedReader(
                            new java.io.InputStreamReader(conn.getInputStream(), "UTF-8"));
                        StringBuilder sb = new StringBuilder();
                        String line;
                        while ((line = reader.readLine()) != null) sb.append(line);
                        reader.close();
                        conn.disconnect();

                        org.json.JSONObject json = new org.json.JSONObject(sb.toString());
                        final String serverVersion = json.getString("version");
                        final String currentVersion = Config.APK_VERSION_NAME;

                        if (compareVersions(serverVersion, currentVersion) > 0) {
                            // Check if we already prompted for this version (SharedPreferences)
                            String promptedVersion = getPreferences(MODE_PRIVATE)
                                .getString("update_prompted_version", "");
                            if (serverVersion.equals(promptedVersion)) {
                                Log.i(TAG, "Already prompted for version " + serverVersion + ", skipping");
                                return;
                            }
                            runOnUiThread(new Runnable() {
                                @Override
                                public void run() {
                                    String sv = serverVersion;
                                    new AlertDialog.Builder(MainActivity.this)
                                        .setTitle("Доступна новая версия")
                                        .setMessage("Версия " + sv + ". Скачать и установить?")
                                        .setPositiveButton("Скачать", new DialogInterface.OnClickListener() {
                                            public void onClick(DialogInterface d, int w) {
                                                downloadAndInstallApk();
                                            }
                                        })
                                        .setNegativeButton("Позже", new DialogInterface.OnClickListener() {
                                            public void onClick(DialogInterface d, int w) {
                                                getPreferences(MODE_PRIVATE).edit()
                                                    .putString("update_prompted_version", sv).apply();
                                            }
                                        })
                                        .setOnCancelListener(new DialogInterface.OnCancelListener() {
                                            public void onCancel(DialogInterface d) {
                                                getPreferences(MODE_PRIVATE).edit()
                                                    .putString("update_prompted_version", sv).apply();
                                            }
                                        })
                                        .show();
                                }
                            });
                        }
                    } catch (Exception e) {
                        Log.i(TAG, "Update check failed (no retry until next launch): " + e.getMessage());
                    }
                }

                private int compareVersions(String a, String b) {
                    String[] pa = a.split("\\.");
                    String[] pb = b.split("\\.");
                    int maxLen = Math.max(pa.length, pb.length);
                    for (int i = 0; i < maxLen; i++) {
                        int na = i < pa.length ? Integer.parseInt(pa[i]) : 0;
                        int nb = i < pb.length ? Integer.parseInt(pb[i]) : 0;
                        if (na > nb) return 1;
                        if (na < nb) return -1;
                    }
                    return 0;
                }
            }).start();
        }

        private void downloadAndInstallApk() {
            Log.i(TAG, "Download and install APK requested");
            new Thread(new Runnable() {
                @Override
                public void run() {
                    try {
                        String targetUrl = Config.TARGET_URL;
                        java.net.URL url = new java.net.URL(targetUrl + "api/v1/apk/download");
                        java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();

                        if (url.getProtocol().equals("https")) {
                            javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                            javax.net.ssl.SSLSocketFactory sf = createEmbeddedCertSSLSocketFactory();
                            if (sf != null) {
                                httpsConn.setSSLSocketFactory(sf);
                            }
                            httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                                public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                            });
                        }
                        conn.setRequestMethod("GET");
                        conn.setRequestProperty("Authorization", "Bearer " + authToken);
                        conn.setConnectTimeout(30000);
                        conn.setReadTimeout(60000);

                        int responseCode = conn.getResponseCode();
                        if (responseCode != 200) {
                            java.io.InputStream es = conn.getErrorStream();
                            String errMsg = "HTTP " + responseCode;
                            if (es != null) {
                                java.io.BufferedReader r = new java.io.BufferedReader(
                                    new java.io.InputStreamReader(es, "UTF-8"));
                                StringBuilder sb = new StringBuilder();
                                String l;
                                while ((l = r.readLine()) != null) sb.append(l);
                                r.close();
                                errMsg = sb.toString();
                            }
                            final String finalErr = errMsg;
                            runOnUiThread(new Runnable() {
                                @Override
                                public void run() {
                                    Toast.makeText(MainActivity.this,
                                        "Ошибка скачивания: " + finalErr, Toast.LENGTH_LONG).show();
                                }
                            });
                            return;
                        }

                        java.io.InputStream is = conn.getInputStream();
                        File cacheDir = getCacheDir();
                        File apkFile = new File(cacheDir, "library-update.apk");
                        FileOutputStream fos = new FileOutputStream(apkFile);
                        byte[] buf = new byte[8192];
                        int len;
                        int total = 0;
                        while ((len = is.read(buf)) != -1) {
                            fos.write(buf, 0, len);
                            total += len;
                        }
                        fos.close();
                        is.close();
                        conn.disconnect();
                        Log.i(TAG, "APK downloaded: " + total + " bytes to " + apkFile.getAbsolutePath());

                        final Uri apkUri = FileProvider.getUriForFile(
                            MainActivity.this,
                            getPackageName() + ".fileprovider",
                            apkFile);

                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                Intent intent = new Intent(Intent.ACTION_VIEW);
                                intent.setDataAndType(apkUri, "application/vnd.android.package-archive");
                                intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                                try {
                                    startActivity(intent);
                                } catch (ActivityNotFoundException e) {
                                    Toast.makeText(MainActivity.this,
                                        "Не удалось запустить установщик APK", Toast.LENGTH_LONG).show();
                                }
                            }
                        });
                    } catch (final Exception e) {
                        Log.e(TAG, "APK download/install error: " + e.getMessage());
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                Toast.makeText(MainActivity.this,
                                    "Ошибка: " + e.getMessage(), Toast.LENGTH_LONG).show();
                            }
                        });
                    }
                }
            }).start();
        }

        @JavascriptInterface
        public void notifyCriticalResourceFailed(String type) {
            Log.i(TAG, "Critical resource failed: " + type);
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    if (!offlineMode) {
                        appendDebug("Critical " + type + " resource failed, loading offline page");
                        startupTimeoutHandler.removeCallbacks(startupTimeoutRunnable);
                        loadOfflinePage();
                    }
                }
            });
        }

        @JavascriptInterface
        public void login(String username, String password, String deviceName, String deviceFingerprint) {
            appendDebug("Bridge login: " + username);
            new Thread(new Runnable() {
                @Override
                public void run() {
                    try {
                        String targetUrl = Config.TARGET_URL;
                        java.net.URL url = new java.net.URL(targetUrl + "api/v1/auth/login");
                        java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();

                        if (url.getProtocol().equals("https")) {
                            javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                            javax.net.ssl.SSLSocketFactory sf = createEmbeddedCertSSLSocketFactory();
                            if (sf != null) {
                                httpsConn.setSSLSocketFactory(sf);
                            }
                            httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                                public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                            });
                        }
                        conn.setRequestMethod("POST");
                        conn.setRequestProperty("Content-Type", "application/json");
                        conn.setRequestProperty("X-Platform", "android");
                        conn.setDoOutput(true);
                        conn.setConnectTimeout(15000);
                        conn.setReadTimeout(30000);

                        String body = "{\"username\":\"" + escapeJson(username) + "\",\"password\":\"" + escapeJson(password)
                                + "\",\"device_name\":\"" + escapeJson(deviceName) + "\",\"device_fingerprint\":\"" + escapeJson(deviceFingerprint) + "\"}";
                        byte[] bodyBytes = body.getBytes("UTF-8");
                        conn.setRequestProperty("Content-Length", String.valueOf(bodyBytes.length));
                        conn.getOutputStream().write(bodyBytes);
                        conn.getOutputStream().flush();
                        conn.getOutputStream().close();
                        appendDebug("Bridge login body_len=" + bodyBytes.length);

                        int responseCode = conn.getResponseCode();
                        java.io.InputStream is = (responseCode >= 200 && responseCode < 300)
                                ? conn.getInputStream() : conn.getErrorStream();
                        java.io.BufferedReader reader = new java.io.BufferedReader(
                                new java.io.InputStreamReader(is, "UTF-8"));
                        StringBuilder response = new StringBuilder();
                        String line;
                        while ((line = reader.readLine()) != null) response.append(line);
                        reader.close();

                        final String result = response.toString();
                        appendDebug("Bridge login HTTP " + responseCode + " len=" + result.length());
                        callAuthBridgeCallback(responseCode, result);
                    } catch (final Exception e) {
                        appendDebug("Bridge login error: " + e.getMessage());
                        callAuthBridgeCallback(0, "{\"error\":\"" + escapeJson(e.getMessage()) + "\"}");
                    }
                }
            }).start();
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

    private static String escapeJson(String s) {
        if (s == null) return "";
        return s.replace("\\", "\\\\").replace("\"", "\\\"")
                .replace("\n", "\\n").replace("\r", "\\r").replace("\t", "\\t");
    }

    private javax.net.ssl.SSLSocketFactory createEmbeddedCertSSLSocketFactory() {
        try {
            java.security.cert.CertificateFactory cf = java.security.cert.CertificateFactory.getInstance("X.509");
            java.io.InputStream certIn = getResources().openRawResource(R.raw.ca_cert);
            java.security.cert.X509Certificate caCert = (java.security.cert.X509Certificate) cf.generateCertificate(certIn);
            certIn.close();

            java.security.KeyStore ks = java.security.KeyStore.getInstance(java.security.KeyStore.getDefaultType());
            ks.load(null, null);
            ks.setCertificateEntry("ca", caCert);

            javax.net.ssl.TrustManagerFactory tmf = javax.net.ssl.TrustManagerFactory.getInstance(
                    javax.net.ssl.TrustManagerFactory.getDefaultAlgorithm());
            tmf.init(ks);

            javax.net.ssl.KeyManager[] keyManagers = null;
            try {
                java.io.InputStream certIn2 = getResources().openRawResource(R.raw.client_cert);
                java.security.KeyStore clientKs = java.security.KeyStore.getInstance("PKCS12");
                clientKs.load(certIn2, Config.CLIENT_CERT_PASSWORD.toCharArray());
                certIn2.close();
                javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
                        javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
                kmf.init(clientKs, Config.CLIENT_CERT_PASSWORD.toCharArray());
                keyManagers = kmf.getKeyManagers();
            } catch (Exception e) {
                appendDebug("Client cert load for bridge skipped: " + e.getMessage());
            }

            javax.net.ssl.SSLContext sslContext = javax.net.ssl.SSLContext.getInstance("TLS");
            sslContext.init(keyManagers, tmf.getTrustManagers(), new java.security.SecureRandom());
            return sslContext.getSocketFactory();
        } catch (Exception e) {
            appendDebug("Embedded cert SSL factory failed: " + e.getMessage());
            return null;
        }
    }

    // ── Generic HTTP proxy via client certificate ──────────────────
    private class HttpProxyBridge {
        @JavascriptInterface
        public String httpRequest(String method, String url, String headersJson, String body) {
            try {
                java.net.URL javaUrl = new java.net.URL(url);
                java.net.HttpURLConnection conn = (java.net.HttpURLConnection) javaUrl.openConnection();

                if (javaUrl.getProtocol().equals("https")) {
                    javax.net.ssl.HttpsURLConnection httpsConn = (javax.net.ssl.HttpsURLConnection) conn;
                    javax.net.ssl.SSLSocketFactory sf = createEmbeddedCertSSLSocketFactory();
                    if (sf != null) {
                        httpsConn.setSSLSocketFactory(sf);
                    }
                    httpsConn.setHostnameVerifier(new javax.net.ssl.HostnameVerifier() {
                        public boolean verify(String hostname, javax.net.ssl.SSLSession session) { return true; }
                    });
                }

                conn.setRequestMethod(method);
                conn.setInstanceFollowRedirects(true);
                conn.setConnectTimeout(30000);
                conn.setReadTimeout(60000);

                if (headersJson != null && !headersJson.isEmpty()) {
                    try {
                        org.json.JSONObject headers = new org.json.JSONObject(headersJson);
                        java.util.Iterator<String> keys = headers.keys();
                        while (keys.hasNext()) {
                            String key = keys.next();
                            if (!key.equalsIgnoreCase("Host")) {
                                conn.setRequestProperty(key, headers.getString(key));
                            }
                        }
                    } catch (Exception ignored) {}
                }

                conn.setRequestProperty("X-Platform", "android");

                if (android.webkit.CookieManager.getInstance() != null) {
                    String cookies = android.webkit.CookieManager.getInstance().getCookie(url);
                    if (cookies != null && !cookies.isEmpty()) {
                        conn.setRequestProperty("Cookie", cookies);
                    }
                }

                if (body != null && !body.isEmpty()
                        && ("POST".equals(method) || "PUT".equals(method) || "PATCH".equals(method) || "DELETE".equals(method))) {
                    conn.setDoOutput(true);
                    java.io.OutputStream os = conn.getOutputStream();
                    os.write(body.getBytes("UTF-8"));
                    os.flush();
                    os.close();
                }

                int status = conn.getResponseCode();

                java.io.InputStream respStream;
                if (status >= 400) {
                    respStream = conn.getErrorStream();
                } else {
                    respStream = conn.getInputStream();
                }

                StringBuilder sb = new StringBuilder();
                if (respStream != null) {
                    java.io.BufferedReader reader = new java.io.BufferedReader(
                            new java.io.InputStreamReader(respStream, "UTF-8"));
                    String line;
                    while ((line = reader.readLine()) != null) sb.append(line);
                    reader.close();
                }

                String respHeaders = "{";
                for (java.util.Map.Entry<String, java.util.List<String>> entry : conn.getHeaderFields().entrySet()) {
                    if (entry.getKey() != null && entry.getValue() != null && !entry.getValue().isEmpty()) {
                        respHeaders += "\"" + escapeJson(entry.getKey()) + "\":\"" + escapeJson(entry.getValue().get(0)) + "\",";
                    }
                }
                if (respHeaders.endsWith(",")) respHeaders = respHeaders.substring(0, respHeaders.length() - 1);
                respHeaders += "}";

                appendDebug("HTTP proxy: " + method + " " + url + " → " + status);

                org.json.JSONObject result = new org.json.JSONObject();
                result.put("status", status);
                result.put("body", sb.toString());
                result.put("headers", new org.json.JSONObject(respHeaders));
                return result.toString();
            } catch (Exception e) {
                appendDebug("HTTP proxy error: " + e.getMessage());
                try {
                    org.json.JSONObject err = new org.json.JSONObject();
                    err.put("status", 0);
                    err.put("body", "{\"error\":\"" + escapeJson(e.getMessage()) + "\"}");
                    err.put("headers", new org.json.JSONObject());
                    return err.toString();
                } catch (Exception e2) {
                    return "{\"status\":0,\"body\":\"{}\",\"headers\":{}}";
                }
            }
        }
    }

    // ── Asset serving helpers ──────────────────────────────────────
    private void loadOfflinePage() {
        try {
            java.io.InputStream is = getAssets().open("www/offline.html");
            java.io.ByteArrayOutputStream buffer = new java.io.ByteArrayOutputStream();
            byte[] data = new byte[1024];
            int nRead;
            while ((nRead = is.read(data, 0, data.length)) != -1) {
                buffer.write(data, 0, nRead);
            }
            buffer.flush();
            is.close();
            String html = buffer.toString("UTF-8");
            webView.loadDataWithBaseURL("file:///android_asset/www/", html, "text/html", "UTF-8", null);
            appendDebug("Loaded offline page from assets");
            offlineMode = true;
        } catch (java.io.IOException e) {
            showError("Failed to load offline page");
            appendDebug("Failed to load offline page: " + e.getMessage());
        }
    }

    private WebResourceResponse serveFromAssets(String assetPath, String mimeType) {
        try {
            InputStream is = getAssets().open(assetPath);
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                Map<String, String> headers = new HashMap<>();
                headers.put("Cache-Control", "no-cache, no-store, must-revalidate");
                return new WebResourceResponse(mimeType, "UTF-8", 200, "OK", headers, is);
            }
            return new WebResourceResponse(mimeType, "UTF-8", is);
        } catch (IOException e) {
            appendDebug("Asset not found: " + assetPath);
            return null;
        }
    }

    private WebResourceResponse serveIndexFromAssets() {
        try {
            String html = readAssetToString("www/index.html");
            html = html.replace("</head>", MOBILE_CSS_TAG + "\n</head>");
            html = html.replace("<body>", ANDROID_BODY + "\n    " + MOBILE_TOP_BAR_INDEX);
            html = html.replace("</body>", ANDROID_JS + "\n</body>");
            InputStream is = new ByteArrayInputStream(html.getBytes("UTF-8"));
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                return new WebResourceResponse("text/html", "UTF-8", 200, "OK", null, is);
            }
            return new WebResourceResponse("text/html", "UTF-8", is);
        } catch (IOException e) {
            appendDebug("Failed to serve index from assets: " + e.getMessage());
            return null;
        }
    }

    private WebResourceResponse serveAdminFromAssets() {
        try {
            String html = readAssetToString("www/admin.html");
            html = html.replace("</head>", MOBILE_CSS_TAG + "\n</head>");
            html = html.replace("<body>", ANDROID_BODY + "\n    " + MOBILE_TOP_BAR_ADMIN);
            html = html.replace("</body>", ANDROID_JS + "\n</body>");
            InputStream is = new ByteArrayInputStream(html.getBytes("UTF-8"));
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                return new WebResourceResponse("text/html", "UTF-8", 200, "OK", null, is);
            }
            return new WebResourceResponse("text/html", "UTF-8", is);
        } catch (IOException e) {
            appendDebug("Failed to serve admin from assets: " + e.getMessage());
            return null;
        }
    }

    private void loadMainPageFromAssets() {
        try {
            String html = readAssetToString("www/index.html");
            html = html.replace("</head>", MOBILE_CSS_TAG + "\n</head>");
            html = html.replace("<body>", ANDROID_BODY + "\n    " + MOBILE_TOP_BAR_INDEX);
            html = html.replace("</body>", ANDROID_JS + "\n</body>");
            webView.loadDataWithBaseURL(TARGET_URL, html, "text/html", "UTF-8", null);
            appendDebug("Main page loaded from assets (no network)");
        } catch (IOException e) {
            appendDebug("Failed to load main page from assets, falling back to network: " + e.getMessage());
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                Map<String, String> headers = new HashMap<>();
                headers.put("X-Platform", "android");
                webView.loadUrl(TARGET_URL, headers);
            } else {
                webView.loadUrl(TARGET_URL);
            }
        }
    }

    private String readAssetToString(String path) throws IOException {
        BufferedReader reader = new BufferedReader(
            new InputStreamReader(getAssets().open(path), "UTF-8"));
        StringBuilder sb = new StringBuilder();
        String line;
        while ((line = reader.readLine()) != null) {
            sb.append(line).append("\n");
        }
        reader.close();
        return sb.toString();
    }

    private String getMimeType(String path) {
        if (path == null) return null;
        if (path.endsWith(".css")) return "text/css";
        if (path.endsWith(".js")) return "application/javascript";
        if (path.endsWith(".svg")) return "image/svg+xml";
        if (path.endsWith(".ico")) return "image/x-icon";
        if (path.endsWith(".png")) return "image/png";
        if (path.endsWith(".json")) return "application/json";
        if (path.endsWith(".woff2")) return "font/woff2";
        if (path.endsWith(".woff")) return "font/woff";
        return null;
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
