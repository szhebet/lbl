package app.library.twa;

import android.app.Activity;
import android.content.Context;
import android.graphics.Color;
import android.net.http.SslError;
import android.os.Build;
import android.os.Bundle;
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
    private static final String TARGET_URL = "https://192.168.95.200:9443/";

    private WebView webView;
    private LinearLayout debugPanel;
    private TextView debugLog;
    private boolean hasError = false;

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
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                appendDebug("Finished: " + url);
                if (!hasError) {
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
                    showError("HTTP Error: " + code + "\n" + desc);
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
                    ks.load(in, "changeit".toCharArray());
                    in.close();

                    Enumeration<String> aliases = ks.aliases();
                    if (!aliases.hasMoreElements()) {
                        appendDebug("No client cert alias found in PKCS12");
                        request.cancel();
                        return;
                    }

                    String alias = aliases.nextElement();
                    PrivateKey privateKey = (PrivateKey) ks.getKey(alias, "changeit".toCharArray());
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
