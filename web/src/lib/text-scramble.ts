/**
 * Text Scramble Utility
 *
 * Produces a terminal-style "decrypting text" animation by revealing characters
 * left-to-right while randomising unrevealed characters from a glitch charset.
 *
 * Usage:
 *   const cancel = scrambleText('AUTHENTICATING…', setText);
 *   // call cancel() to stop early if needed
 */

const CHARSET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789@#$%&';

/**
 * Animate target string being "decrypted" left-to-right.
 *
 * @param target     Final string to reveal
 * @param onUpdate   Called on every interval tick with the current scrambled string
 * @param durationMs Total animation duration in milliseconds (default 800)
 * @param intervalMs Tick interval in milliseconds (default 50)
 * @returns          Cancel function — call to stop the animation early
 */
export function scrambleText(
  target: string,
  onUpdate: (s: string) => void,
  durationMs: number = 800,
  intervalMs: number = 50
): () => void {
  const start = Date.now();
  const id = setInterval(() => {
    const elapsed = Date.now() - start;
    const progress = Math.min(elapsed / durationMs, 1);
    const revealed = Math.floor(target.length * progress);
    let s = target.slice(0, revealed);
    for (let i = revealed; i < target.length; i++) {
      s += CHARSET[Math.floor(Math.random() * CHARSET.length)];
    }
    onUpdate(s);
    if (progress >= 1) {
      clearInterval(id);
      onUpdate(target);
    }
  }, intervalMs);
  return () => clearInterval(id);
}
