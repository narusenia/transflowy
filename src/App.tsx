import { useEffect, useState } from "react";
import { Library, Settings, type Meta } from "@/mygo";

export function App() {
  const [dir, setDir] = useState("");
  const [recordings, setRecordings] = useState<Meta[]>([]);

  useEffect(() => {
    Settings.get().then((s) => setDir(s.libraryDir));
    Library.list().then(setRecordings);
  }, []);

  return (
    <main className="p-6 text-sm">
      <h1 className="text-lg font-semibold">transflowy</h1>
      <p className="text-muted-foreground">
        {dir} · {recordings.length} 件
      </p>
    </main>
  );
}
