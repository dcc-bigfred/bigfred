import { useWakeLock } from "../hooks/useWakeLock";

// WakeLockKeeper mounts once at the app root and prevents the device
// screen from dimming while BigFred is open in the foreground.
export default function WakeLockKeeper() {
  useWakeLock(true);
  return null;
}
