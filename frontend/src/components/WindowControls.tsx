import { useEffect, useState, type CSSProperties } from "react";
import { Copy, Minus, Square, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { t } from "@/lib/i18n";
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
      <Button
        variant="ghost"
        type="button"
        className="h-10 w-12 rounded-none text-muted-foreground hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent"
        onClick={() => windowApi.minimise()}
        aria-label={t("Minimize")}
      >
        <Minus className="size-4" />
      </Button>
      <Button
        variant="ghost"
        type="button"
        className="h-10 w-12 rounded-none text-muted-foreground hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent"
        onClick={() => void toggleMaximise()}
        aria-label={maximised ? t("Restore") : t("Maximize")}
      >
        {maximised ? <Copy className="size-3.5" /> : <Square className="size-3.5" />}
      </Button>
      <Button
        variant="ghost"
        type="button"
        className="h-10 w-12 rounded-none text-muted-foreground hover:bg-destructive hover:text-destructive-foreground dark:hover:bg-destructive"
        onClick={() => void windowApi.close()}
        aria-label={t("Close")}
      >
        <X className="size-4" />
      </Button>
    </div>
  );
}
