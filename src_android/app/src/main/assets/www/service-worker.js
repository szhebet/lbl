var CACHE_NAME = 'library-cache-v3';
var SHELL_URLS = [
  '/',
  '/static/css/style.css',
  '/static/css/mobile.css',
  '/static/js/app.js',
  '/static/js/auth.js',
  '/static/js/import.js',
  '/static/js/offline.js',
  '/static/favicon.svg'
];

self.addEventListener('install', function(e) {
  e.waitUntil(
    caches.open(CACHE_NAME).then(function(cache) {
      return cache.addAll(SHELL_URLS);
    }).then(function() {
      return self.skipWaiting();
    })
  );
});

self.addEventListener('activate', function(e) {
  e.waitUntil(
    Promise.all([
      self.clients.claim(),
      caches.keys().then(function(names) {
        return Promise.all(
          names.filter(function(n) { return n !== CACHE_NAME; })
            .map(function(n) { return caches.delete(n); })
        );
      })
    ])
  );
});

function shouldCache(url) {
  var path = url.pathname;
  if (path === '/' || path === '') return true;
  if (path.startsWith('/static/')) return true;
  return false;
}

function isNavigation(url) {
  var p = url.pathname;
  return p === '/' || p === '' || p === '/admin' || p === '/admin/';
}

self.addEventListener('fetch', function(e) {
  var url = new URL(e.request.url);
  if (!shouldCache(url)) return;

  if (isNavigation(url)) {
    e.respondWith(
      caches.match(e.request).then(function(cached) {
        var networkPromise = fetch(e.request).then(function(response) {
          if (response && response.ok) {
            var clone = response.clone();
            caches.open(CACHE_NAME).then(function(cache) {
              cache.put(e.request, clone);
            });
          }
          return response;
        }).catch(function() {
          return null;
        });

        if (cached) {
          networkPromise.catch(function() {});
          return cached;
        }

        return networkPromise.then(function(response) {
          return response || new Response(
            '<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Домашняя библиотека</title><meta name="viewport" content="width=device-width,initial-scale=1.0"><style>body{font-family:sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f5f5f5;color:#333}.offline-msg{text-align:center;padding:20px}.offline-msg h1{font-size:1.5em;margin-bottom:10px}.offline-msg p{color:#666}</style></head><body><div class="offline-msg"><h1>Сервер недоступен</h1><p>Проверьте соединение и попробуйте снова.</p></div></body></html>',
            { status: 503, headers: { 'Content-Type': 'text/html; charset=utf-8' } }
          );
        });
      })
    );
    return;
  }

  e.respondWith(
    caches.match(e.request, { ignoreSearch: true }).then(function(cached) {
      var networkPromise = fetch(e.request).then(function(response) {
        if (response && response.ok) {
          var clone = response.clone();
          caches.open(CACHE_NAME).then(function(cache) {
            cache.put(e.request, clone);
          });
        }
        return response;
      }).catch(function() {
        return cached;
      });

      if (cached) {
        networkPromise.catch(function() {});
        return cached;
      }

      return networkPromise;
    })
  );
});
