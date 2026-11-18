import { useEffect, useState, type CSSProperties } from "react";
import { Copy, Minus, Square, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { windowApi } from "@/lib/wails";

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;
const noDragStyle = { "--wails-draggable": "no-drag" } as CSSProperties;

export function TitleBar({ pageTitle }: { pageTitle: string }) {
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
    <header
      className="flex h-9 shrink-0 select-none items-center justify-between border-b bg-background pl-4"
      style={dragStyle}
      onDoubleClick={() => void toggleMaximise()}
    >
      <div className="flex min-w-0 items-center gap-2">
        <span className="text-xs font-medium">Rapoo Control</span>
        <span className="truncate text-xs text-muted-foreground">{pageTitle}</span>
      </div>

      <div className="flex h-full items-center" style={noDragStyle}>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-11 rounded-none"
          onClick={() => windowApi.minimise()}
          aria-label="最小化"
        >
          <Minus className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-11 rounded-none"
          onClick={() => void toggleMaximise()}
          aria-label={maximised ? "元に戻す" : "最大化"}
        >
          {maximised ? <Copy className="size-3.5" /> : <Square className="size-3.5" />}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-11 rounded-none hover:bg-destructive hover:text-destructive-foreground"
          onClick={() => void windowApi.close()}
          aria-label="閉じる"
        >
          <X className="size-4" />
        </Button>
      </div>
    </header>
  );
}
