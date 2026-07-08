package app.library.twa;

import android.content.Context;
import android.content.SharedPreferences;
import android.os.Build;
import android.security.KeyPairGeneratorSpec;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import android.util.Log;

import java.math.BigInteger;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.util.Calendar;
import java.util.Locale;

import javax.crypto.Cipher;
import javax.security.auth.x500.X500Principal;

public class TokenStore {
    private static final String TAG = "TokenStore";
    private static final String KEYSTORE_ALIAS = "library_app_refresh_key";
    private static final String PREFS_NAME = "library_app_secure";
    private static final String KEY_REFRESH_TOKEN = "refresh_token";

    private final Context context;
    private final SharedPreferences prefs;

    public TokenStore(Context context) {
        this.context = context;
        this.prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
        ensureKey();
    }

    private void ensureKey() {
        try {
            KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
            ks.load(null);
            if (ks.containsAlias(KEYSTORE_ALIAS)) return;

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                KeyPairGenerator kpg = KeyPairGenerator.getInstance(
                        KeyProperties.KEY_ALGORITHM_RSA, "AndroidKeyStore");
                kpg.initialize(new KeyGenParameterSpec.Builder(
                        KEYSTORE_ALIAS,
                        KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                        .setBlockModes(KeyProperties.BLOCK_MODE_ECB)
                        .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_RSA_PKCS1)
                        .setKeySize(2048)
                        .build());
                kpg.generateKeyPair();
            } else {
                Calendar start = Calendar.getInstance();
                Calendar end = Calendar.getInstance();
                end.add(Calendar.YEAR, 100);
                KeyPairGeneratorSpec spec = new KeyPairGeneratorSpec.Builder(context)
                        .setAlias(KEYSTORE_ALIAS)
                        .setSubject(new X500Principal("CN=LibraryApp"))
                        .setSerialNumber(BigInteger.ONE)
                        .setStartDate(start.getTime())
                        .setEndDate(end.getTime())
                        .build();
                KeyPairGenerator kpg = KeyPairGenerator.getInstance("RSA", "AndroidKeyStore");
                kpg.initialize(spec);
                kpg.generateKeyPair();
            }
        } catch (Exception e) {
            Log.e(TAG, "Failed to create KeyStore key", e);
        }
    }

    private PublicKey getPublicKey() {
        try {
            KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
            ks.load(null);
            if (!ks.containsAlias(KEYSTORE_ALIAS)) return null;
            return ks.getCertificate(KEYSTORE_ALIAS).getPublicKey();
        } catch (Exception e) {
            Log.e(TAG, "Failed to get public key", e);
            return null;
        }
    }

    private PrivateKey getPrivateKey() {
        try {
            KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
            ks.load(null);
            if (!ks.containsAlias(KEYSTORE_ALIAS)) return null;
            return (PrivateKey) ks.getKey(KEYSTORE_ALIAS, null);
        } catch (Exception e) {
            Log.e(TAG, "Failed to get private key", e);
            return null;
        }
    }

    public void storeRefreshToken(String token) {
        try {
            PublicKey publicKey = getPublicKey();
            if (publicKey == null) {
                Log.w(TAG, "No public key, storing token without encryption");
                prefs.edit().putString(KEY_REFRESH_TOKEN, token).apply();
                return;
            }
            Cipher cipher = Cipher.getInstance("RSA/ECB/PKCS1Padding");
            cipher.init(Cipher.ENCRYPT_MODE, publicKey);
            byte[] encrypted = cipher.doFinal(token.getBytes("UTF-8"));
            String encoded = Base64.encodeToString(encrypted, Base64.NO_WRAP);
            prefs.edit().putString(KEY_REFRESH_TOKEN, encoded).apply();
            Log.i(TAG, "Refresh token stored securely");
        } catch (Exception e) {
            Log.e(TAG, "Failed to store refresh token", e);
            prefs.edit().putString(KEY_REFRESH_TOKEN, token).apply();
        }
    }

    public String getRefreshToken() {
        try {
            String stored = prefs.getString(KEY_REFRESH_TOKEN, null);
            if (stored == null || stored.isEmpty()) return null;

            PrivateKey privateKey = getPrivateKey();
            if (privateKey == null) {
                return stored;
            }

            byte[] encrypted = Base64.decode(stored, Base64.NO_WRAP);
            Cipher cipher = Cipher.getInstance("RSA/ECB/PKCS1Padding");
            cipher.init(Cipher.DECRYPT_MODE, privateKey);
            byte[] decrypted = cipher.doFinal(encrypted);
            return new String(decrypted, "UTF-8");
        } catch (Exception e) {
            Log.e(TAG, "Failed to decrypt refresh token, trying plaintext", e);
            String stored = prefs.getString(KEY_REFRESH_TOKEN, null);
            return stored;
        }
    }

    public void clearRefreshToken() {
        prefs.edit().remove(KEY_REFRESH_TOKEN).apply();
        Log.i(TAG, "Refresh token cleared");
    }
}
