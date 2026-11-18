import { useEffect, useState, type CSSProperties } from "react";
import { Copy, Minus, Square, X } from "lucide-react";
import { windowApi } from "@/lib/wails";

const noDragStyle = { "--wails-draggable": "no-drag" } as CSSProperties;

export function WindowControls() {
  const [maximised, setMaximised] = useState(false);

  useEffect(() => {
    let active = true;
    const update = async () => {
      const value = await windowApi.isMaximised();
      if (active) setMaximised(value);
    };
    void update();
    window.addEventListener("resize", update);
    return () => {
      active = false;
      window.removeEventListener("resize", update);
    };
  }, []);

  const toggleMaximise = async () => {
    setMaximised(await windowApi.toggleMaximise());
  };

  return (
    <div
      className="absolute right-0 top-0 z-30 flex h-10 items-center"
      style={noDragStyle}
      onDoubleClick={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        className="flex h-10 w-12 items-center justify-center text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        onClick={() => windowApi.minimise()}
        aria-label="最小化"
      >
        <Minus className="size-4" />
      </button>
      <button
        type="button"
        className="flex h-10 w-12 items-center justify-center text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        onClick={() => void toggleMaximise()}
        aria-label={maximised ? "元に戻す" : "最大化"}
      >
        {maximised ? <Copy className="size-3.5" /> : <Square className="size-3.5" />}
      </button>
      <button
        type="button"
        className="flex h-10 w-12 items-center justify-center text-muted-foreground transition-colors hover:bg-destructive hover:text-destructive-foreground"
        onClick={() => void windowApi.close()}
        aria-label="閉じる"
      >
        <X className="size-4" />
      </button>
    </div>
  );
}
