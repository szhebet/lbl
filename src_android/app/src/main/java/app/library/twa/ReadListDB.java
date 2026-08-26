package app.library.twa;

import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.database.sqlite.SQLiteOpenHelper;
import android.util.Log;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;

public class ReadListDB extends SQLiteOpenHelper {
    private static final String TAG = "ReadListDB";
    private static final String DB_NAME = "readlist.db";
    private static final int DB_VERSION = 4;

    private static final String TABLE_ITEMS = "readlist_items";
    private static final String TABLE_QUEUE = "offline_queue";
    private static final String TABLE_OFFERS = "readlist_offers";

    public ReadListDB(Context context) {
        super(context, DB_NAME, null, DB_VERSION);
    }

    @Override
    public void onCreate(SQLiteDatabase db) {
        db.execSQL(
            "CREATE TABLE " + TABLE_ITEMS + " (" +
            "id TEXT PRIMARY KEY," +
            "listname TEXT NOT NULL DEFAULT 'default'," +
            "bookname TEXT NOT NULL DEFAULT ''," +
            "author TEXT NOT NULL DEFAULT ''," +
            "priority INTEGER NOT NULL DEFAULT 0," +
            "author_id INTEGER," +
            "book_id INTEGER," +
            "user_id INTEGER NOT NULL DEFAULT 0," +
            "comment TEXT NOT NULL DEFAULT ''," +
            "status TEXT NOT NULL DEFAULT 'Не заполнено'," +
            "looking_for TEXT NOT NULL DEFAULT 'Нет'," +
            "deleted INTEGER NOT NULL DEFAULT 0," +
            "created_at TEXT," +
            "updated_at TEXT," +
            "synced_at TEXT," +
            "format_name TEXT DEFAULT ''," +
            "on_shelf INTEGER DEFAULT 0," +
            "edition_id INTEGER" +
            ")"
        );
        db.execSQL(
            "CREATE TABLE " + TABLE_QUEUE + " (" +
            "id INTEGER PRIMARY KEY AUTOINCREMENT," +
            "operation TEXT NOT NULL," +
            "item_id TEXT NOT NULL," +
            "body TEXT," +
            "created_at TEXT NOT NULL DEFAULT (datetime('now'))" +
            ")"
        );
        createOffersTable(db);
    }

    private static void createOffersTable(SQLiteDatabase db) {
        db.execSQL(
            "CREATE TABLE IF NOT EXISTS " + TABLE_OFFERS + " (" +
            "id INTEGER PRIMARY KEY," +
            "read_list_id TEXT NOT NULL," +
            "title TEXT NOT NULL DEFAULT ''," +
            "authors TEXT NOT NULL DEFAULT ''," +
            "source_url TEXT NOT NULL DEFAULT ''," +
            "remote_edition_id INTEGER NOT NULL DEFAULT 0," +
            "edition_id INTEGER," +
            "received_at TEXT," +
            "linked INTEGER NOT NULL DEFAULT 0," +
            "user_id INTEGER NOT NULL DEFAULT 0" +
            ")"
        );
    }

    @Override
    public void onUpgrade(SQLiteDatabase db, int oldVersion, int newVersion) {
        if (oldVersion < 2) {
            db.execSQL("ALTER TABLE " + TABLE_ITEMS + " ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0");
        }
        if (oldVersion < 3) {
            db.execSQL("ALTER TABLE " + TABLE_ITEMS + " ADD COLUMN looking_for TEXT NOT NULL DEFAULT 'Нет'");
        }
        if (oldVersion < 4) {
            createOffersTable(db);
        }
    }

    public void replaceAll(String jsonArray) {
        SQLiteDatabase db = getWritableDatabase();
        db.beginTransaction();
        try {
            db.delete(TABLE_ITEMS, null, null);
            JSONArray arr = new JSONArray(jsonArray);
            for (int i = 0; i < arr.length(); i++) {
                JSONObject item = arr.getJSONObject(i);
                ContentValues cv = itemToValues(item);
                db.insertWithOnConflict(TABLE_ITEMS, null, cv, SQLiteDatabase.CONFLICT_REPLACE);
            }
            db.setTransactionSuccessful();
        } catch (Exception e) {
            Log.e(TAG, "replaceAll error: " + e.getMessage());
        } finally {
            db.endTransaction();
        }
    }

    public String queryAll(String listname, String bookname, String author, String statusFilter) {
        SQLiteDatabase db = getReadableDatabase();
        List<String> args = new ArrayList<>();
        StringBuilder sql = new StringBuilder("SELECT * FROM " + TABLE_ITEMS + " WHERE 1=1");

        if (listname != null && !listname.isEmpty()) {
            sql.append(" AND listname = ?");
            args.add(listname);
        }
        if (bookname != null && !bookname.isEmpty()) {
            sql.append(" AND bookname LIKE ?");
            args.add("%" + bookname + "%");
        }
        if (author != null && !author.isEmpty()) {
            sql.append(" AND author LIKE ?");
            args.add("%" + author + "%");
        }
        if (statusFilter != null && !statusFilter.isEmpty()) {
            String[] statuses = statusFilter.split(",");
            sql.append(" AND status IN (");
            for (int i = 0; i < statuses.length; i++) {
                if (i > 0) sql.append(",");
                sql.append("?");
                args.add(statuses[i].trim());
            }
            sql.append(")");
        }

        sql.append(" ORDER BY priority DESC");

        Cursor c = db.rawQuery(sql.toString(), args.toArray(new String[0]));
        JSONArray result = new JSONArray();
        while (c.moveToNext()) {
            JSONObject item = new JSONObject();
            try {
                item.put("id", getString(c, "id"));
                item.put("listname", getString(c, "listname"));
                item.put("bookname", getString(c, "bookname"));
                item.put("author", getString(c, "author"));
                item.put("priority", c.getInt(c.getColumnIndexOrThrow("priority")));
                item.put("comment", getString(c, "comment"));
                item.put("status", getString(c, "status"));
                item.put("looking_for", getString(c, "looking_for"));
                item.put("deleted", c.getInt(c.getColumnIndexOrThrow("deleted")) != 0);
                item.put("created_at", getString(c, "created_at"));
                item.put("updated_at", getString(c, "updated_at"));
                item.put("synced_at", getString(c, "synced_at"));
                item.put("format_name", getString(c, "format_name"));
                item.put("on_shelf", c.getInt(c.getColumnIndexOrThrow("on_shelf")) != 0);
                if (!c.isNull(c.getColumnIndexOrThrow("user_id")))
                    item.put("user_id", c.getInt(c.getColumnIndexOrThrow("user_id")));
                if (!c.isNull(c.getColumnIndexOrThrow("edition_id")))
                    item.put("edition_id", c.getInt(c.getColumnIndexOrThrow("edition_id")));
                if (!c.isNull(c.getColumnIndexOrThrow("author_id")))
                    item.put("author_id", c.getInt(c.getColumnIndexOrThrow("author_id")));
                if (!c.isNull(c.getColumnIndexOrThrow("book_id")))
                    item.put("book_id", c.getInt(c.getColumnIndexOrThrow("book_id")));
                result.put(item);
            } catch (Exception e) {
                Log.e(TAG, "Error reading row: " + e.getMessage());
            }
        }
        c.close();
        return result.toString();
    }

    public void upsertItem(String jsonString) {
        try {
            JSONObject item = new JSONObject(jsonString);
            ContentValues cv = itemToValues(item);
            SQLiteDatabase db = getWritableDatabase();
            db.insertWithOnConflict(TABLE_ITEMS, null, cv, SQLiteDatabase.CONFLICT_REPLACE);
        } catch (Exception e) {
            Log.e(TAG, "upsertItem error: " + e.getMessage());
        }
    }

    public void deleteItem(String id) {
        SQLiteDatabase db = getWritableDatabase();
        db.delete(TABLE_ITEMS, "id = ?", new String[]{id});
    }

    public void clearAll() {
        SQLiteDatabase db = getWritableDatabase();
        db.delete(TABLE_ITEMS, null, null);
    }

    // ── Offers cache (fed_offers mirror, server → client only) ──

    /** Replaces the WHOLE offers cache with the given array (JSON). */
    public void replaceAllOffers(String jsonArray) {
        SQLiteDatabase db = getWritableDatabase();
        db.beginTransaction();
        try {
            db.delete(TABLE_OFFERS, null, null);
            JSONArray arr = new JSONArray(jsonArray);
            for (int i = 0; i < arr.length(); i++) {
                JSONObject offer = arr.getJSONObject(i);
                ContentValues cv = offerToValues(offer);
                db.insertWithOnConflict(TABLE_OFFERS, null, cv, SQLiteDatabase.CONFLICT_REPLACE);
            }
            db.setTransactionSuccessful();
        } catch (Exception e) {
            Log.e(TAG, "replaceAllOffers error: " + e.getMessage());
        } finally {
            db.endTransaction();
        }
    }

    public String queryAllOffers() {
        SQLiteDatabase db = getReadableDatabase();
        Cursor c = db.rawQuery("SELECT * FROM " + TABLE_OFFERS +
            " ORDER BY read_list_id ASC, linked DESC, received_at DESC, id DESC", null);
        JSONArray result = new JSONArray();
        while (c.moveToNext()) {
            JSONObject offer = new JSONObject();
            try {
                offer.put("id", c.getLong(c.getColumnIndexOrThrow("id")));
                offer.put("read_list_id", getString(c, "read_list_id"));
                offer.put("title", getString(c, "title"));
                offer.put("authors", getString(c, "authors"));
                offer.put("source_url", getString(c, "source_url"));
                offer.put("remote_edition_id", c.getLong(c.getColumnIndexOrThrow("remote_edition_id")));
                if (!c.isNull(c.getColumnIndexOrThrow("edition_id")))
                    offer.put("edition_id", c.getInt(c.getColumnIndexOrThrow("edition_id")));
                offer.put("received_at", getString(c, "received_at"));
                offer.put("linked", c.getInt(c.getColumnIndexOrThrow("linked")) != 0);
                offer.put("user_id", c.getInt(c.getColumnIndexOrThrow("user_id")));
                result.put(offer);
            } catch (Exception e) {
                Log.e(TAG, "Error reading offer row: " + e.getMessage());
            }
        }
        c.close();
        return result.toString();
    }

    private ContentValues offerToValues(JSONObject offer) throws Exception {
        ContentValues cv = new ContentValues();
        cv.put("id", offer.optLong("id", 0));
        cv.put("read_list_id", offer.optString("read_list_id", ""));
        cv.put("title", offer.optString("title", ""));
        cv.put("authors", offer.optString("authors", ""));
        cv.put("source_url", offer.optString("source_url", ""));
        cv.put("remote_edition_id", offer.optLong("remote_edition_id", 0));
        if (offer.has("edition_id") && !offer.isNull("edition_id"))
            cv.put("edition_id", offer.getInt("edition_id"));
        cv.put("received_at", offer.isNull("received_at") ? "" : offer.optString("received_at"));
        cv.put("linked", offer.optBoolean("linked", false) ? 1 : 0);
        cv.put("user_id", offer.optInt("user_id", 0));
        return cv;
    }

    public void enqueue(String operation, String itemId, String body) {
        SQLiteDatabase db = getWritableDatabase();
        ContentValues cv = new ContentValues();
        cv.put("operation", operation);
        cv.put("item_id", itemId);
        cv.put("body", body);
        db.insert(TABLE_QUEUE, null, cv);
    }

    public void enqueueDelete(String itemId) {
        SQLiteDatabase db = getWritableDatabase();
        db.delete(TABLE_ITEMS, "id = ?", new String[]{itemId});
        ContentValues cv = new ContentValues();
        cv.put("operation", "delete");
        cv.put("item_id", itemId);
        db.insert(TABLE_QUEUE, null, cv);
    }

    public String getPendingQueue() {
        SQLiteDatabase db = getReadableDatabase();
        Cursor c = db.rawQuery("SELECT * FROM " + TABLE_QUEUE + " ORDER BY id ASC", null);
        JSONArray result = new JSONArray();
        while (c.moveToNext()) {
            JSONObject entry = new JSONObject();
            try {
                entry.put("id", c.getInt(c.getColumnIndexOrThrow("id")));
                entry.put("operation", getString(c, "operation"));
                entry.put("item_id", getString(c, "item_id"));
                entry.put("body", getString(c, "body"));
                entry.put("created_at", getString(c, "created_at"));
                result.put(entry);
            } catch (Exception e) {
                Log.e(TAG, "Error reading queue: " + e.getMessage());
            }
        }
        c.close();
        return result.toString();
    }

    public int getPendingCount() {
        SQLiteDatabase db = getReadableDatabase();
        Cursor c = db.rawQuery("SELECT COUNT(*) FROM " + TABLE_QUEUE, null);
        int count = 0;
        if (c.moveToFirst()) count = c.getInt(0);
        c.close();
        return count;
    }

    public void clearQueue() {
        SQLiteDatabase db = getWritableDatabase();
        db.delete(TABLE_QUEUE, null, null);
    }

    public void dequeue(int queueId) {
        SQLiteDatabase db = getWritableDatabase();
        db.delete(TABLE_QUEUE, "id = ?", new String[]{String.valueOf(queueId)});
    }

    private ContentValues itemToValues(JSONObject item) throws Exception {
        ContentValues cv = new ContentValues();
        cv.put("id", item.optString("id", ""));
        cv.put("listname", item.optString("listname", "default"));
        cv.put("bookname", item.optString("bookname", ""));
        cv.put("author", item.optString("author", ""));
        cv.put("priority", item.optInt("priority", 0));
        cv.put("comment", item.optString("comment", ""));
        cv.put("status", item.optString("status", "Не заполнено"));
        cv.put("looking_for", item.optString("looking_for", "Нет"));
        cv.put("deleted", item.optBoolean("deleted", false) ? 1 : 0);
        cv.put("created_at", item.isNull("created_at") ? "" : item.optString("created_at"));
        cv.put("updated_at", item.isNull("updated_at") ? "" : item.optString("updated_at"));
        cv.put("synced_at", item.isNull("synced_at") ? "" : item.optString("synced_at"));
        cv.put("format_name", item.optString("format_name", ""));
        cv.put("user_id", item.optInt("user_id", 0));
        if (item.has("author_id") && !item.isNull("author_id"))
            cv.put("author_id", item.getInt("author_id"));
        if (item.has("book_id") && !item.isNull("book_id"))
            cv.put("book_id", item.getInt("book_id"));
        if (item.has("edition_id") && !item.isNull("edition_id"))
            cv.put("edition_id", item.getInt("edition_id"));
        cv.put("on_shelf", item.optBoolean("on_shelf", false) ? 1 : 0);
        return cv;
    }

    private String getString(Cursor c, String col) {
        int idx = c.getColumnIndexOrThrow(col);
        return c.isNull(idx) ? "" : c.getString(idx);
    }
}
