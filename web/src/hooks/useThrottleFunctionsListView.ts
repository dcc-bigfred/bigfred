import { useCallback, useState } from "react";

const STORAGE_KEY = "bigfred.throttle.functionsAsList";

function readStoredFunctionsAsList(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

function writeStoredFunctionsAsList(value: boolean): void {
  try {
    if (value) {
      localStorage.setItem(STORAGE_KEY, "1");
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // Quota or private mode — ignore.
  }
}

/** Persisted Throttle preference: show function buttons as a compact list. */
export function useThrottleFunctionsListView(): {
  functionsAsList: boolean;
  setFunctionsAsList: (value: boolean) => void;
} {
  const [functionsAsList, setFunctionsAsListState] = useState(
    readStoredFunctionsAsList,
  );

  const setFunctionsAsList = useCallback((value: boolean) => {
    writeStoredFunctionsAsList(value);
    setFunctionsAsListState(value);
  }, []);

  return { functionsAsList, setFunctionsAsList };
}
