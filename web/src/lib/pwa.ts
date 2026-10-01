// Service Worker and PWA Auto-Update Utility

type PWAInstallListener = (canInstall: boolean) => void;
type PWAUpdateListener = () => void;

let deferredInstallPrompt: any = null;
const installListeners = new Set<PWAInstallListener>();
const updateListeners = new Set<PWAUpdateListener>();

let refreshing = false;

export function registerServiceWorker() {
  if (typeof window !== 'undefined' && 'serviceWorker' in navigator && (import.meta as any).env?.MODE !== 'test') {
    window.addEventListener('load', () => {
      navigator.serviceWorker
        .register('/sw.js')
        .then((reg) => {
          console.log('[PWA] Service Worker registered:', reg.scope);

          // 1. Immediately check for update
          reg.update().catch(() => {});

          // 2. Check for updates whenever user returns to the app / refocuses
          document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'visible') {
              reg.update().catch(() => {});
            }
          });

          window.addEventListener('focus', () => {
            reg.update().catch(() => {});
          });

          // 3. Periodic check every 5 minutes
          setInterval(() => {
            reg.update().catch(() => {});
          }, 5 * 60 * 1000);

          // 4. Handle newly found worker
          reg.onupdatefound = () => {
            const installingWorker = reg.installing;
            if (installingWorker) {
              installingWorker.onstatechange = () => {
                if (installingWorker.state === 'installed') {
                  if (navigator.serviceWorker.controller) {
                    console.log('[PWA] New version installed and ready.');
                    notifyUpdateListeners();
                  } else {
                    console.log('[PWA] App is cached for offline use.');
                  }
                }
              };
            }
          };
        })
        .catch((err) => {
          console.error('[PWA] Service Worker registration error:', err);
        });

      // 5. When the active service worker changes, reload seamlessly once
      navigator.serviceWorker.addEventListener('controllerchange', () => {
        if (!refreshing) {
          refreshing = true;
          console.log('[PWA] Controller changed; reloading with latest version.');
          window.location.reload();
        }
      });
    });

    // Listen for the beforeinstallprompt event
    window.addEventListener('beforeinstallprompt', (e: any) => {
      e.preventDefault();
      deferredInstallPrompt = e;
      notifyInstallListeners(true);
      console.log('[PWA] beforeinstallprompt captured');
    });

    // Listen for successful installation
    window.addEventListener('appinstalled', () => {
      deferredInstallPrompt = null;
      notifyInstallListeners(false);
      console.log('[PWA] App successfully installed to home screen');
    });
  }
}

export function isPWAInstallable(): boolean {
  return !!deferredInstallPrompt;
}

export function isRunningStandalone(): boolean {
  if (typeof window === 'undefined') return false;
  return (
    window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as any).standalone === true ||
    document.referrer.includes('android-app://')
  );
}

export function isIOS(): boolean {
  if (typeof window === 'undefined') return false;
  return /iPad|iPhone|iPod/.test(navigator.userAgent) && !(window as any).MSStream;
}

export async function promptPWAInstall(): Promise<'accepted' | 'dismissed' | 'unavailable'> {
  if (!deferredInstallPrompt) {
    return 'unavailable';
  }

  deferredInstallPrompt.prompt();
  const choiceResult = await deferredInstallPrompt.userChoice;
  deferredInstallPrompt = null;
  notifyInstallListeners(false);
  return choiceResult.outcome;
}

export function onPWAInstallChange(listener: PWAInstallListener): () => void {
  installListeners.add(listener);
  listener(!!deferredInstallPrompt);
  return () => {
    installListeners.delete(listener);
  };
}

export function onPWAUpdate(listener: PWAUpdateListener): () => void {
  updateListeners.add(listener);
  return () => {
    updateListeners.delete(listener);
  };
}

function notifyInstallListeners(canInstall: boolean) {
  installListeners.forEach((fn) => fn(canInstall));
}

function notifyUpdateListeners() {
  updateListeners.forEach((fn) => fn());
}
