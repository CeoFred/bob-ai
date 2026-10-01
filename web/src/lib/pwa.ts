// Service Worker and PWA Installation Utility

type PWAInstallListener = (canInstall: boolean) => void;

let deferredInstallPrompt: any = null;
const installListeners = new Set<PWAInstallListener>();

export function registerServiceWorker() {
  if (typeof window !== 'undefined' && 'serviceWorker' in navigator && (import.meta as any).env?.MODE !== 'test') {
    window.addEventListener('load', () => {
      navigator.serviceWorker
        .register('/sw.js')
        .then((reg) => {
          console.log('[PWA] Service Worker registered with scope:', reg.scope);

          // Check for SW updates
          reg.onupdatefound = () => {
            const installingWorker = reg.installing;
            if (installingWorker) {
              installingWorker.onstatechange = () => {
                if (installingWorker.state === 'installed') {
                  if (navigator.serviceWorker.controller) {
                    console.log('[PWA] New content is available; please refresh.');
                  } else {
                    console.log('[PWA] Content is cached for offline use.');
                  }
                }
              };
            }
          };
        })
        .catch((err) => {
          console.error('[PWA] Service Worker registration failed:', err);
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
      console.log('[PWA] App successfully installed');
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

function notifyInstallListeners(canInstall: boolean) {
  installListeners.forEach((fn) => fn(canInstall));
}
