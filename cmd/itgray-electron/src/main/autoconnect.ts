// cmd/itgray-electron/src/main/autoconnect.ts

/**
 * createAutoConnectClaim returns a one-shot claim for an app launch: the
 * first call reports true, every later one false.
 *
 * Auto-connect is decided in the renderer, which is the wrong place to
 * remember that it already happened. Closing the window to the tray destroys
 * the WebContents, so re-opening from the tray loads the renderer fresh with
 * its once-per-session guard reset — and the app reconnected each time the
 * window came back, even though no launch occurred. The main process outlives
 * every window, so it owns the decision and the renderer asks it.
 *
 * A brand new claim per launch is what makes a genuine restart auto-connect
 * again: quitting the app takes the claim with it.
 */
export function createAutoConnectClaim(): () => boolean {
  let available = true;
  return () => {
    if (!available) return false;
    available = false;
    return true;
  };
}
