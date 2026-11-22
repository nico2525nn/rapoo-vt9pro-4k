import type { ReactNode } from "react";

export function SettingRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-13 items-center justify-between gap-4 border-b py-2.5 first:pt-0 last:border-0 last:pb-0">
      <div className="min-w-0 flex flex-col gap-0.5">
        <span className="text-sm">{label}</span>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      <div className="flex w-44 max-w-[50%] shrink-0 justify-end [&>*]:max-w-full">{children}</div>
    </div>
  );
}
