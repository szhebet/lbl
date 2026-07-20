# Keep JavaScript bridge methods exposed via @JavascriptInterface
# These are only invoked from JS (reflection) and would otherwise be
# stripped/renamed by R8 in release builds, breaking the WebView bridges.
-keepattributes *Annotation*

-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

-keep class app.library.twa.TokenBridge { *; }
-keep class app.library.twa.MainActivity$TokenBridge { *; }
-keep class app.library.twa.MainActivity$ReadListBridge { *; }
-keep class app.library.twa.MainActivity$FileImportBridge { *; }

# Keep TokenStore / ReadListDB (used via bridge, avoid obfuscation issues)
-keep class app.library.twa.TokenStore { *; }
-keep class app.library.twa.ReadListDB { *; }
