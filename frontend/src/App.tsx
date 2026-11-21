import {useSyncExternalStore} from "react";
import StudentDashboard from "./StudentDashboard";
import {PublicPortfolio} from "./Portfolio";

function watchHash(listener: () => void) {
  window.addEventListener("hashchange", listener);
  return () => window.removeEventListener("hashchange", listener);
}

export default function App() {
  const hash = useSyncExternalStore(watchHash, () => window.location.hash);
  const shareID = /^#portfolio\/([0-9a-f-]{36})$/i.exec(hash)?.[1];
  return shareID ? <PublicPortfolio key={shareID} id={shareID}/> : <StudentDashboard/>;
}
