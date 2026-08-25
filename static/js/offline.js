(function() {
    function getBridge() {
        return window.AndroidReadListDB || null;
    }

    function debug(msg) {
        if (window.appDebug) window.appDebug('[offline] ' + msg);
    }

    function isAndroid() {
        return document.body && document.body.classList.contains('android');
    }

    function getAuthHeaders() {
        var headers = { 'Content-Type': 'application/json' };
        var token = localStorage.getItem('auth_token') || '';
        if (token) headers['Authorization'] = 'Bearer ' + token;
        return headers;
    }

    function isOnline() {
        return typeof navigator !== 'undefined' && navigator.onLine !== false;
    }

    function getCurrentUserId() {
        try {
            var stored = localStorage.getItem('auth_user');
            if (stored) {
                var user = JSON.parse(stored);
                return user && user.id ? user.id : null;
            }
        } catch(e) {}
        if (window.authUser && window.authUser.id) return window.authUser.id;
        return null;
    }

    function getCurrentUsername() {
        try {
            var stored = localStorage.getItem('auth_user');
            if (stored) {
                var user = JSON.parse(stored);
                return user && user.username ? user.username : null;
            }
        } catch(e) {}
        if (window.authUser && window.authUser.username) return window.authUser.username;
        return null;
    }

    function updateIndicator(status) {
        var btn = document.getElementById('mobileUserBtn');
        if (!btn) return;
        btn.classList.remove('sync-ok', 'sync-syncing', 'sync-offline');
        btn.classList.add('sync-' + status);
    }

    function updatePendingCount() {
        var el = document.getElementById('pendingCount');
        var btn = document.getElementById('syncReadlistBtn');
        if (!window.ReadListStore) {
            if (el) el.style.display = 'none';
            if (btn) btn.style.display = 'none';
            return;
        }
        var cnt = ReadListStore.countDirty();
        if (el) {
            if (cnt > 0) {
                el.style.display = 'inline';
                el.textContent = '\u23F3 ' + cnt + ' \u043E\u0436\u0438\u0434\u0430\u044E\u0442 \u0441\u0438\u043D\u0445\u0440\u043E\u043D\u0438\u0437\u0430\u0446\u0438\u0438';
            } else {
                el.style.display = 'none';
            }
        }
        if (btn) {
            btn.style.display = cnt > 0 && isOnline() ? 'inline-block' : 'none';
        }
    }

    // ── ReadListStore ──────────────────────────────────────────
    window.ReadListStore = {
        _cache: null,

        _loadFromBridge() {
            this._cache = [];
            var bridge = getBridge();
            if (!bridge) { debug('_loadFromBridge: bridge not available'); return; }
            try {
                var raw = bridge.queryAll('', '', '', '');
                var allItems = JSON.parse(raw);
                var uid = getCurrentUserId();
                debug('_loadFromBridge: ' + allItems.length + ' total items from SQLite, user=' + uid);
                // Normalize legacy rows where JSON null was stored as the literal string "null"
                for (var n = 0; n < allItems.length; n++) {
                    if (allItems[n].synced_at === 'null') allItems[n].synced_at = '';
                    if (allItems[n].updated_at === 'null') allItems[n].updated_at = '';
                    if (allItems[n].created_at === 'null') allItems[n].created_at = '';
                }
                if (uid !== null) {
                    this._cache = allItems.filter(function(i) {
                        if (i.user_id !== uid) return false;
                        debug('_loadFromBridge: loaded item id=' + i.id + ' user_id=' + i.user_id + ' deleted=' + i.deleted + ' synced_at=' + i.synced_at);
                        return true;
                    });
                } else {
                    this._cache = allItems;
                }
                debug('_loadFromBridge: ' + this._cache.length + ' items for user ' + uid);
            } catch(e) {
                debug('SQLite load failed: ' + e.message);
            }
        },

        _ensureCache() {
            if (!this._cache) this._loadFromBridge();
        },

        _saveToBridge() {
            var bridge = getBridge();
            if (!bridge) return;
            try {
                bridge.replaceAll(JSON.stringify(this._cache));
            } catch(e) {
                debug('SQLite replaceAll failed: ' + e.message);
            }
        },

        _syncOne(item) {
            var bridge = getBridge();
            if (!bridge) return;
            try {
                bridge.upsertItem(JSON.stringify(item));
            } catch(e) {
                debug('SQLite upsertItem failed: ' + e.message);
            }
        },

        _removeOne(id) {
            var bridge = getBridge();
            if (!bridge) return;
            try {
                bridge.deleteItem(id);
            } catch(e) {
                debug('SQLite deleteItem failed: ' + e.message);
            }
        },

        // Public API
        countDirty() {
            this._ensureCache();
            var uid = getCurrentUserId();
            var cnt = 0;
            for (var i = 0; i < this._cache.length; i++) {
                var item = this._cache[i];
                // Only count dirty for current user
                if (item.user_id !== uid) continue;
                if (item.updated_at && (!item.synced_at || item.updated_at > item.synced_at)) {
                    cnt++;
                }
            }
            return cnt;
        },

        getAll() {
            this._ensureCache();
            var uid = getCurrentUserId();
            var items = this._cache.filter(function(i) { return i.user_id === uid; });
            return items.sort(function(a,b) { return (b.priority||0) - (a.priority||0); });
        },

        query(listname, bookname, author, status) {
            this._ensureCache();
            var uid = getCurrentUserId();
            var items = this._cache.filter(function(i) { return i.user_id === uid && !i.deleted; });
            if (listname) items = items.filter(function(i) { return i.listname === listname; });
            if (bookname) items = items.filter(function(i) { return (i.bookname||'').toLowerCase().indexOf(bookname.toLowerCase()) !== -1; });
            if (author) items = items.filter(function(i) { return (i.author||'').toLowerCase().indexOf(author.toLowerCase()) !== -1; });
            if (status) {
                var statuses = status.split(',');
                items = items.filter(function(i) { return statuses.indexOf(i.status) !== -1; });
            }
            items.sort(function(a,b) { return (b.priority||0) - (a.priority||0); });
            return items;
        },

        getById(id) {
            this._ensureCache();
            var uid = getCurrentUserId();
            for (var i = 0; i < this._cache.length; i++) {
                if (this._cache[i].id === id && this._cache[i].user_id === uid) return this._cache[i];
            }
            return null;
        },

        replaceAll(items) {
            var uid = getCurrentUserId();
            // Remove existing items for current user, keep others
            this._ensureCache();
            this._cache = this._cache.filter(function(i) { return i.user_id !== uid; });
            // Add new items with user_id set
            var now = new Date().toISOString();
            for (var i = 0; i < items.length; i++) {
                items[i].user_id = uid;
                if (!items[i].synced_at) items[i].synced_at = items[i].updated_at || now;
                this._cache.push(items[i]);
            }
            this._saveToBridge();
            debug('replaceAll: ' + items.length + ' items for user ' + uid);
        },

        upsert(item) {
            this._ensureCache();
            var uid = getCurrentUserId();
            item.user_id = uid;
            var now = new Date().toISOString();
            item.updated_at = now;
            var idx = -1;
            for (var i = 0; i < this._cache.length; i++) {
                if (this._cache[i].id === item.id && this._cache[i].user_id === uid) { idx = i; break; }
            }
            if (idx >= 0) {
                if (this._cache[idx].created_at && !item.created_at) item.created_at = this._cache[idx].created_at;
                this._cache[idx] = item;
            } else {
                if (!item.created_at) item.created_at = now;
                this._cache.push(item);
            }
            this._syncOne(item);
            debug('upsert: ' + item.id + ' for user ' + uid);
        },

        remove(id) {
            this._ensureCache();
            var uid = getCurrentUserId();
            this._cache = this._cache.filter(function(i) { return !(i.id === id && i.user_id === uid); });
            // Still delete from SQLite even if user_id doesn't match (cleanup)
            this._removeOne(id);
            debug('remove: ' + id);
        },

        markSynced(id, syncedAt) {
            var item = this.getById(id);
            if (!item) return;
            item.synced_at = syncedAt || new Date().toISOString();
            this._syncOne(item);
        },

        markAllSynced() {
            var uid = getCurrentUserId();
            var now = new Date().toISOString();
            for (var i = 0; i < this._cache.length; i++) {
                if (this._cache[i].user_id !== uid) continue;
                this._cache[i].synced_at = this._cache[i].updated_at || now;
                this._syncOne(this._cache[i]);
            }
            debug('markAllSynced for user ' + uid);
        },

        getDirty() {
            this._ensureCache();
            var uid = getCurrentUserId();
            var dirty = [];
            for (var i = 0; i < this._cache.length; i++) {
                var item = this._cache[i];
                if (item.user_id !== uid) continue;
                if (item.updated_at && (!item.synced_at || item.updated_at > item.synced_at)) {
                    dirty.push(item);
                }
            }
            dirty.sort(function(a,b) { return (a.updated_at||'') < (b.updated_at||'') ? -1 : 1; });
            return dirty;
        },

        clearByUser(uid) {
            this._ensureCache();
            // Purge other users' data on account switch: keep only the
            // current user's items (and locally-created ones with no user_id yet).
            this._cache = this._cache.filter(function(i) {
                return !i.user_id || i.user_id === uid;
            });
            this._saveToBridge();
            debug('clearByUser: ' + uid);
            // Offers are purged the same way (server → client cache only).
            if (ReadListStore.offersSupported()) {
                ReadListStore._ensureOffers();
                ReadListStore._offersCache = ReadListStore._offersCache.filter(function(o) {
                    return !o.user_id || o.user_id === uid;
                });
                ReadListStore._saveOffersToBridge();
            }
        },

        // ── Offers cache (fed_offers mirror) ────────────────────
        // Book offers are synced ONLY server → client and are never pushed.
        // Requires an APK bridge with queryAllOffers/replaceAllOffers — older
        // APK builds simply keep today's behaviour (no cached offers).

        offersSupported() {
            var bridge = getBridge();
            return !!(bridge &&
                typeof bridge.queryAllOffers === 'function' &&
                typeof bridge.replaceAllOffers === 'function');
        },

        _loadOffersFromBridge() {
            this._offersCache = [];
            var bridge = getBridge();
            if (!bridge || !this.offersSupported()) return;
            try {
                var all = JSON.parse(bridge.queryAllOffers());
                var uid = getCurrentUserId();
                this._offersCache = all.filter(function(o) { return o.user_id === uid; });
                debug('_loadOffersFromBridge: ' + this._offersCache.length + ' offers for user ' + uid);
            } catch(e) {
                debug('offers load failed: ' + e.message);
            }
        },

        _ensureOffers() {
            if (!this._offersCache) this._loadOffersFromBridge();
        },

        _saveOffersToBridge() {
            var bridge = getBridge();
            if (!bridge || !this.offersSupported()) return;
            try {
                bridge.replaceAllOffers(JSON.stringify(this._offersCache));
            } catch(e) {
                debug('replaceAllOffers failed: ' + e.message);
            }
        },

        // Cached offers for one read list, sorted like the server does:
        // linked first, then newest.
        getOffers(rlId) {
            this._ensureOffers();
            var uid = getCurrentUserId();
            return this._offersCache
                .filter(function(o) { return o.user_id === uid && o.read_list_id === rlId; })
                .sort(function(a,b) {
                    if (a.linked !== b.linked) return a.linked ? -1 : 1;
                    var ta = a.received_at || '', tb = b.received_at || '';
                    if (ta !== tb) return ta > tb ? -1 : 1;
                    return (b.id||0) - (a.id||0);
                });
        },

        // Replace cached offers of ONE read list (used when the SPA loads them
        // online or after a successful link).
        replaceOffers(rlId, items) {
            if (!this.offersSupported()) return;
            this._ensureOffers();
            var uid = getCurrentUserId();
            this._offersCache = this._offersCache.filter(function(o) {
                return !(o.user_id === uid && o.read_list_id === rlId);
            });
            for (var i = 0; i < items.length; i++) {
                var o = {};
                for (var k in items[i]) if (items[i].hasOwnProperty(k)) o[k] = items[i][k];
                o.read_list_id = rlId;
                o.user_id = uid;
                this._offersCache.push(o);
            }
            this._saveOffersToBridge();
            debug('replaceOffers: ' + items.length + ' for list ' + rlId);
        },

        // Full replacement from the sync pull: drop ALL current user's cached
        // offers and store what the server returned (other users' rows kept).
        replaceAllOffers(items) {
            if (!this.offersSupported()) return;
            this._ensureOffers();
            var uid = getCurrentUserId();
            this._offersCache = this._offersCache.filter(function(o) { return o.user_id !== uid; });
            for (var i = 0; i < items.length; i++) {
                var o = {};
                for (var k in items[i]) if (items[i].hasOwnProperty(k)) o[k] = items[i][k];
                if (!o.read_list_id) continue;
                o.user_id = uid;
                this._offersCache.push(o);
            }
            this._saveOffersToBridge();
            debug('replaceAllOffers: ' + items.length + ' for user ' + uid);
        }
    };

    // ── SyncService ────────────────────────────────────────────
    window.SyncService = {
        _syncing: false,

        isSyncing() {
            return this._syncing;
        },

        async sync() {
            if (this._syncing) {
                debug('sync already in progress, skipping');
                return;
            }
            var uid = getCurrentUserId();
            if (uid === null) {
                debug('no authenticated user, sync skipped');
                updateIndicator('offline');
                return;
            }

            this._syncing = true;
            updateIndicator('syncing');
            debug('sync started for user ' + uid);

            var anyError = false;

            // Step 1: Push dirty items for current user
            var dirty = ReadListStore.getDirty();
            debug('push: ' + dirty.length + ' dirty items for user ' + uid);

            for (var i = 0; i < dirty.length; i++) {
                var item = dirty[i];
                if (item.user_id !== uid) continue;

                var applyServerItem = function(serverItem) {
                    if (!serverItem) return;
                    var localItem = ReadListStore.getById(serverItem.id) || ReadListStore.getById(item.id);
                    if (localItem) {
                        var serverId = localItem.id;
                        for (var k in serverItem) {
                            if (serverItem.hasOwnProperty(k) && k !== 'user_id') {
                                localItem[k] = serverItem[k];
                            }
                        }
                        localItem.user_id = uid;
                        localItem.id = serverId;
                        ReadListStore._syncOne(localItem);
                        ReadListStore.markSynced(localItem.id, serverItem.updated_at || localItem.updated_at);
                    } else {
                        // Last resort: mark the original item as synced so we don't keep retrying
                        ReadListStore.markSynced(item.id, item.updated_at);
                        debug('applyServerItem: no local item found for ' + serverItem.id + ' or ' + item.id + ', marked synced');
                    }
                };

                try {
                    var putResp = await fetch('/api/v1/user/readlist/' + item.id, {
                        method: 'PUT',
                        headers: getAuthHeaders(),
                        body: JSON.stringify(item)
                    });
                    if (putResp.ok || putResp.status === 201) {
                        try { var putBody = await putResp.json(); applyServerItem(putBody); }
                        catch(e) { ReadListStore.markSynced(item.id, item.updated_at); }
                        debug('push UPDATE ok: ' + item.id);
                    } else if (putResp.status === 409) {
                        try {
                            var conflictBody = await putResp.json();
                            if (conflictBody.server_item) {
                                applyServerItem(conflictBody.server_item);
                                debug('push CONFLICT: server newer, adopted server state: ' + item.id);
                            } else {
                                ReadListStore.markSynced(item.id, item.updated_at);
                            }
                        } catch(e) {
                            ReadListStore.markSynced(item.id, item.updated_at);
                            debug('push CONFLICT: failed to parse server state for ' + item.id);
                        }
                    } else if (putResp.status === 404) {
                        var postResp = await fetch('/api/v1/user/readlist', {
                            method: 'POST',
                            headers: getAuthHeaders(),
                            body: JSON.stringify(item)
                        });
                        if (postResp.ok || postResp.status === 201) {
                            try { var postBody = await postResp.json(); applyServerItem(postBody); }
                            catch(e) { ReadListStore.markSynced(item.id, item.updated_at); }
                            debug('push CREATE ok: ' + item.id);
                        } else if (postResp.status === 500) {
                            // Try to GET the item — if it exists on server (dup UUID), adopt server state
                            try {
                                var getResp = await fetch('/api/v1/user/readlist/' + item.id, { headers: getAuthHeaders() });
                                if (getResp.ok) {
                                    var getBody = await getResp.json();
                                    applyServerItem(getBody);
                                    debug('push: exists on server (dup), adopted: ' + item.id);
                                } else {
                                    anyError = true;
                                    debug('push CREATE 500, item not on server, keeping dirty: ' + item.id);
                                }
                            } catch(e2) {
                                anyError = true;
                                debug('push CREATE 500, get failed, keeping dirty: ' + item.id);
                            }
                        } else {
                            debug('push CREATE failed: ' + item.id + ' status=' + postResp.status);
                        }
                    } else {
                        anyError = true;
                        debug('push UPDATE failed: ' + item.id + ' status=' + putResp.status);
                    }
                } catch(e) {
                    anyError = true;
                    debug('push network error for ' + item.id + ': ' + e.message);
                }
            }

            // Step 2: Pull all server items for current user (paginated)
            debug('pull: fetching server items for user ' + uid);
            try {
                var serverIds = {};
                var localIds = {};
                var localItems = ReadListStore.getAll();
                for (var li = 0; li < localItems.length; li++) {
                    localIds[localItems[li].id] = localItems[li];
                }

                var pullPage = 0;
                var pullLimit = 100;
                var pullOk = true;
                while (true) {
                    var pullResp = await fetch('/api/v1/user/readlist?limit=' + pullLimit + '&offset=' + (pullPage * pullLimit), {
                        headers: getAuthHeaders()
                    });
                    if (!pullResp.ok) { pullOk = false; break; }
                    var data = await pullResp.json();
                    var serverItems = data.items || [];
                    if (serverItems.length === 0) break;

                    for (var si = 0; si < serverItems.length; si++) {
                        var sItem = serverItems[si];
                        serverIds[sItem.id] = true;
                        sItem.user_id = uid;
                        var lItem = localIds[sItem.id];

                        if (!lItem) {
                            sItem.synced_at = sItem.updated_at;
                            ReadListStore.upsert(sItem);
                            ReadListStore.markSynced(sItem.id, sItem.updated_at);
                            debug('pull new: ' + sItem.id);
                        } else {
                            var sTime = sItem.updated_at || '';
                            var lSynced = lItem.synced_at || '';
                            if (sTime > lSynced) {
                                var lUpdated = lItem.updated_at || '';
                                if (lUpdated > sTime) {
                                    debug('pull: local ' + sItem.id + ' is newer, keeping local');
                                } else {
                                    sItem.synced_at = sItem.updated_at;
                                    ReadListStore.upsert(sItem);
                                    ReadListStore.markSynced(sItem.id, sItem.updated_at);
                                    debug('pull updated: ' + sItem.id);
                                }
                            }
                        }
                    }

                    pullPage++;
                }

                // Cleanup only runs when the whole pull succeeded — a failed pull must
                // never delete local items (serverIds would be incomplete/empty).
                if (pullOk) {
                    for (var li2 = 0; li2 < localItems.length; li2++) {
                        var lItem2 = localItems[li2];
                        if (lItem2.user_id !== uid) continue;
                        if (!serverIds[lItem2.id]) {
                            var isDirty = lItem2.updated_at && (!lItem2.synced_at || lItem2.updated_at > lItem2.synced_at);
                            if (isDirty) {
                                debug('pull: local dirty item missing on server: ' + lItem2.id);
                            } else if (lItem2.synced_at) {
                                ReadListStore.remove(lItem2.id);
                                debug('pull removed local: ' + lItem2.id);
                            } else {
                                debug('pull: local item never synced, keeping: ' + lItem2.id);
                            }
                        }
                    }
                } else {
                    anyError = true;
                    debug('pull: request failed, local items kept');
                }

                debug('pull complete for user ' + uid);
            } catch(e) {
                anyError = true;
                debug('pull network error: ' + e.message);
            }

            // Step 2b: Pull book offers for current user's read lists.
            // ONE-WAY only (server → client) — offers are never pushed and
            // never queued; linking an offer requires a live connection.
            if (ReadListStore.offersSupported && ReadListStore.offersSupported()) {
                debug('pull offers: fetching server offers for user ' + uid);
                try {
                    var offResp = await fetch('/api/v1/user/readlist/offers', { headers: getAuthHeaders() });
                    if (offResp.ok) {
                        var offData = await offResp.json();
                        ReadListStore.replaceAllOffers((offData && offData.items) || []);
                    } else {
                        anyError = true;
                        debug('pull offers failed: status=' + offResp.status);
                    }
                } catch(e3) {
                    anyError = true;
                    debug('pull offers network error: ' + e3.message);
                }
            }

            this._syncing = false;
            updateIndicator(anyError ? 'offline' : 'ok');
            updatePendingCount();
            debug('sync complete for user ' + uid);

            if (isAndroid() && !anyError) {
                var readlistTab = document.getElementById('readlist');
                if (readlistTab && readlistTab.classList.contains('active') && window.loadReadlist) {
                    loadReadlist();
                }
                if (window.loadReadlistNames) loadReadlistNames();
            }
        }
    };

    // ── Initialization ─────────────────────────────────────────
    function init() {
        if (!isAndroid()) return;
        debug('offline init');
        var uid = getCurrentUserId();
        if (uid === null) {
            debug('no user, waiting for auth-changed');
            return;
        }

        // Pre-warm the cache
        ReadListStore._ensureCache();

        // Check for stale data from a different user
        var hasOtherUserData = false;
        var cache = ReadListStore._cache || [];
        for (var i = 0; i < cache.length; i++) {
            if (cache[i].user_id && cache[i].user_id !== uid) {
                hasOtherUserData = true;
                break;
            }
        }
        if (hasOtherUserData) {
            debug('clearing stale data for previous user, current uid=' + uid);
            ReadListStore.clearByUser(uid);
            ReadListStore._loadFromBridge();
            ReadListStore.clearByUser(uid);
        }

        updatePendingCount();

        // Trigger sync on init if user is logged in (handles page reload after login)
        if (window.SyncService && !SyncService.isSyncing()) {
            SyncService.sync();
        }
    }

    // Retry init on auth change
    window.addEventListener('auth-changed', function() {
        debug('auth-changed event received');
        init();
        // Sync after login
        var uid = getCurrentUserId();
        if (uid !== null && window.SyncService && !SyncService.isSyncing()) {
            setTimeout(function() { SyncService.sync(); }, 500);
        }
    });

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    // Expose for sync button
    document.addEventListener('click', function(e) {
        var syncBtn = e.target.closest('#syncReadlistBtn');
        if (syncBtn && window.SyncService) {
            e.preventDefault();
            SyncService.sync();
        }
        var pendingEl = e.target.closest('#pendingCount');
        if (pendingEl && window.SyncService) {
            e.preventDefault();
            SyncService.sync();
        }
    });

    debug('offline.js loaded');
})();
