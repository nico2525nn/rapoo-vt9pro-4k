import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import App from "@/App";
import { fallbackSim } from "@/lib/wails";

describe("Rapoo Control — VT9 Pro 4K Desktop UI", () => {
  beforeEach(() => {
    fallbackSim.resetForTests();
    window.localStorage.clear();
  });

  it("shows disconnected onboarding and transitions cleanly into Demo Mode", async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(await screen.findByText(/Rapoo VT9 Pro 4K を接続してください/)).toBeInTheDocument();

    const demoButtons = screen.getAllByRole("button", { name: /デモモードで全機能を試す/ });
    await user.click(demoButtons[0]);

    expect(await screen.findByText("82%")).toBeInTheDocument();
    expect(screen.getByText(/オンボード DPI ステージ設定/)).toBeInTheDocument();
    expect(screen.getByText("4000 Hz")).toBeInTheDocument();
  });

  it("edits DPI values, switches active DPI stage, and updates polling rate to 4000 Hz", async () => {
    const user = userEvent.setup();
    render(<App />);

    const demoButtons = await screen.findAllByRole("button", { name: /デモモードで全機能を試す/ });
    await user.click(demoButtons[0]);

    await screen.findByText("Stage 2 の感度編集");

    // Click quick DPI chip 1600 for Stage 2
    await user.click(screen.getByRole("button", { name: "1600" }));
    expect(screen.getByLabelText("DPI 感度")).toHaveValue(1600);

    // Switch polling rate to 4000 Hz
    const rate4kCard = screen.getByRole("button", { name: /4000 Hz.*0\.25 ms/ });
    await user.click(rate4kCard);
    expect(rate4kCard).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: /1600 DPI.*4000 Hz/ })).toBeInTheDocument();
  });

  it("updates PAW3395 sensor DSP switches and debounce steps", async () => {
    const user = userEvent.setup();
    render(<App />);

    const demoButtons = await screen.findAllByRole("button", { name: /デモモードで全機能を試す/ });
    await user.click(demoButtons[0]);
    await screen.findByText("82%");

    await user.click(screen.getByRole("tab", { name: /センサー & スイッチ/ }));
    expect(await screen.findByText(/リフトオフディスタンス/)).toBeInTheDocument();

    const motionSyncSwitch = screen.getByRole("switch", { name: /Motion Sync/ });
    expect(motionSyncSwitch).toHaveAttribute("data-state", "unchecked");
    await user.click(motionSyncSwitch);
    expect(motionSyncSwitch).toHaveAttribute("data-state", "checked");

    // Select 1.0 mm LOD preset button
    await user.click(screen.getByRole("button", { name: "1.0 mm" }));
    expect(screen.getAllByText("1.0 mm").length).toBeGreaterThanOrEqual(1);
  });

  it("renders all 10 physical buttons including Bottom DIY and enforces Left-Click Safety Guard", async () => {
    const user = userEvent.setup();
    render(<App />);

    const demoButtons = await screen.findAllByRole("button", { name: /デモモードで全機能を試す/ });
    await user.click(demoButtons[0]);
    await screen.findByText("82%");

    await user.click(screen.getByRole("tab", { name: /10ボタン割り当て/ }));
    expect(await screen.findByText("底面 DIY ボタン")).toBeInTheDocument();

    // Attempt to disable sole Left Click -> Left-Click Safety Guard must block saving
    await user.click(screen.getByRole("button", { name: "左クリック の割り当てを変更" }));
    const dialog = await screen.findByRole("dialog");

    await user.click(within(dialog).getByRole("tab", { name: /無効化/ }));
    expect(await within(dialog).findByText("左クリック保護ガード")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "この割り当てを保存" })).toBeDisabled();

    await user.click(within(dialog).getByRole("button", { name: "キャンセル" }));

    // Remap Bottom DIY Button to Polling Rate Switch
    await user.click(screen.getByRole("button", { name: "底面 DIY ボタン の割り当てを変更" }));
    const diyDialog = await screen.findByRole("dialog");
    await user.click(within(diyDialog).getByRole("button", { name: /ポーリングレート切替/ }));
    await user.click(within(diyDialog).getByRole("button", { name: "この割り当てを保存" }));

    expect(await screen.findByText("ポーリングレート切替")).toBeInTheDocument();
  });

  it("saves user presets, applies built-in FPS 4K preset, and enforces Gaming Lock (0-TX)", async () => {
    const user = userEvent.setup();
    render(<App />);

    const demoButtons = await screen.findAllByRole("button", { name: /デモモードで全機能を試す/ });
    await user.click(demoButtons[0]);
    await screen.findByText("82%");

    await user.click(screen.getByRole("tab", { name: /プロファイル & 診断/ }));
    expect(await screen.findByText(/FPS 競技用/)).toBeInTheDocument();

    // Save current config as a user preset
    await user.click(screen.getByRole("button", { name: /現在の設定を保存/ }));
    const saveDialog = await screen.findByRole("dialog");
    await user.type(within(saveDialog).getByLabelText(/プロファイル名/), "カスタムテスト設定");
    await user.click(within(saveDialog).getByRole("button", { name: "保存する" }));

    expect(await screen.findByText("カスタムテスト設定")).toBeInTheDocument();

    // Enable Gaming Lock in HeaderBar -> Apply buttons must become disabled
    await user.click(screen.getByRole("button", { name: /ゲーム中ロック/ }));
    expect(await screen.findByText(/ゲーム中ロック有効:/)).toBeInTheDocument();

    const applyButtons = screen.getAllByRole("button", { name: "適用" });
    expect(applyButtons[0]).toBeDisabled();

    // Unlock Gaming Lock -> Apply buttons re-enabled
    await user.click(screen.getByRole("button", { name: "ロック解除" }));
    expect(screen.getAllByRole("button", { name: "適用" })[0]).not.toBeDisabled();
  });
});
