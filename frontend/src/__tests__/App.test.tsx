import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import App from "@/App";
import { setLanguage } from "@/lib/i18n";
import { fallbackSim } from "@/lib/wails";

async function enterDemo(user: UserEvent) {
  await user.click(await screen.findByRole("button", { name: "Demo mode" }));
  expect(await within(screen.getByRole("complementary")).findByText(/82%$/)).toBeInTheDocument();
}

async function selectOption(user: UserEvent, label: string, option: string) {
  await user.click(screen.getByLabelText(label));
  await user.click(await screen.findByRole("option", { name: option }));
}

describe("Rapoo Control", () => {
  beforeEach(() => {
    setLanguage("en");
    fallbackSim.resetForTests();
    window.localStorage.clear();
  });

  it("starts disconnected and enters demo mode", async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(await screen.findByText("Mouse not connected")).toBeInTheDocument();
    await enterDemo(user);

    expect(within(screen.getByRole("complementary")).getByRole("status")).toHaveTextContent(/^Demo · .* · 82%$/);
    expect(screen.getByRole("button", { name: "DPI" })).toBeInTheDocument();
    expect(screen.getByText("Stage 1")).toBeInTheDocument();
  });

  it("saves DPI and changes the polling rate", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    const xInput = screen.getByLabelText("Stage 2 X");
    await user.clear(xInput);
    await user.type(xInput, "1600");
    await user.tab();
    const sidebar = screen.getByRole("complementary");
    expect(await within(sidebar).findByText(/1,600 DPI/)).toBeInTheDocument();

    await selectOption(user, "Polling rate", "4,000 Hz");
    expect(within(sidebar).getByText(/4,000 Hz/)).toBeInTheDocument();

    const axisSwitch = screen.getByRole("switch", { name: "Independent X/Y" });
    await user.click(axisSwitch);
    const yInput = await screen.findByLabelText("Stage 2 Y");
    await user.clear(yInput);
    await user.type(yInput, "1200");
    await user.tab();
    expect(await within(sidebar).findByText(/1,600\/1,200 DPI/)).toBeInTheDocument();

    await user.click(axisSwitch);
    expect(screen.queryByLabelText("Stage 2 Y")).not.toBeInTheDocument();
    expect(within(sidebar).getByText(/1,600 DPI/)).toBeInTheDocument();
  });

  it("updates sensor settings", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "Sensor" }));

    const motionSync = screen.getByRole("switch", { name: "Motion Sync" });
    expect(motionSync).toHaveAttribute("data-state", "unchecked");
    await user.click(motionSync);
    expect(motionSync).toHaveAttribute("data-state", "checked");

    await selectOption(user, "Press debounce", "1 ms");
    expect(screen.getByLabelText("Press debounce")).toHaveTextContent("1 ms");

    await selectOption(user, "Lift-off distance", "1.0 mm");
    expect(screen.getByLabelText("Lift-off distance")).toHaveTextContent("1.0 mm");
  });

  it("edits button mappings and protects the last left click", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "Buttons" }));
    expect(screen.getByText("Bottom DIY Button")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Change Left Click" }));
    const guardDialog = await screen.findByRole("dialog");
    await user.click(within(guardDialog).getByLabelText("Action"));
    await user.click(await screen.findByRole("option", { name: "Disabled" }));
    expect(within(guardDialog).getByText(/At least one button must keep Left Click/)).toBeInTheDocument();
    expect(within(guardDialog).getByRole("button", { name: "Save" })).toBeDisabled();
    await user.click(within(guardDialog).getByRole("button", { name: "Cancel" }));

    await user.click(screen.getByRole("button", { name: "Change Bottom DIY Button" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByLabelText("Action"));
    await user.click(await screen.findByRole("option", { name: "Polling rate switch" }));
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Polling rate switch")).toBeInTheDocument();
  });

  it("saves presets and locks presets in passive mode", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "Profiles" }));
    expect(screen.getByText("No saved presets.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Save" }));
    const saveDialog = await screen.findByRole("dialog");
    await user.type(within(saveDialog).getByLabelText("Name"), "Custom");
    await user.click(within(saveDialog).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Custom")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Settings" }));
    await selectOption(user, "Background polling", "In-game lock (0-TX)");
    expect(within(screen.getByRole("complementary")).getByText(/0-TX/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Profiles" }));
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Settings" }));
    await selectOption(user, "Background polling", "Normal");
    await user.click(screen.getByRole("button", { name: "Profiles" }));
    expect(screen.getByRole("button", { name: "Apply" })).toBeEnabled();
  });

  it("starts in the persisted Japanese preference", async () => {
    fallbackSim.setLanguage("ja");
    render(<App />);

    expect(await screen.findByText("マウスが接続されていません")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "設定" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "センサー" })).toBeInTheDocument();
  });

  it("switches to Japanese and back from Settings", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "Settings" }));
    await selectOption(user, "Language", "日本語");
    expect(screen.getByText("デバイス")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ボタン" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "プロファイル" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "DPI" }));
    expect(await screen.findByText("ステージ 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "ボタン" }));
    expect((await screen.findAllByText("左クリック")).length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "設定" }));
    await selectOption(user, "言語", "English");
    expect(screen.getByText("Device")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "DPI" }));
    expect(await screen.findByText("Stage 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Buttons" }));
    expect((await screen.findAllByText("Left Click")).length).toBeGreaterThan(0);
  });
});
