import { useEffect, useState } from "react";
import { onLocaleChange } from "mygo-runtime";
import { messages, messagesIn, type Messages } from "./locales";

/** The app's messages in the current language; the page redraws when it changes. */
export function useMessages(): Messages {
  const [m, setM] = useState<Messages>(messages);
  useEffect(() => onLocaleChange((tag) => setM(messagesIn(tag))), []);
  return m;
}
