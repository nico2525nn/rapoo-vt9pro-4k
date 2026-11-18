import { render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import App from "@/App";
import { fallbackSim } from "@/lib/wails";

async function enterDemo(user: UserEvent) {
  await user.click(await screen.findByRole("button", { name: "デモモード" }));
  await screen.findByText(/電池 82%/);
}

async function selectOption(user: UserEvent, label: string, option: string) {
  await user.click(screen.getByLabelText(label));
  await user.click(await screen.findByRole("option", { name: option }));
}

describe("Rapoo Control", () => {
  beforeEach(() => {
    fallbackSim.resetForTests();
    window.localStorage.clear();
  });

  it("starts disconnected and enters demo mode", async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(await screen.findByText("マウスが接続されていません")).toBeInTheDocument();
    await enterDemo(user);

    expect(screen.getByText(/デモモード · .*電池 82%/)).toBeInTheDocument();
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
    expect(await within(sidebar).findByText("1600 DPI")).toBeInTheDocument();

    await selectOption(user, "ポーリングレート", "4000 Hz");
    expect(within(sidebar).getByText("4000 Hz")).toBeInTheDocument();
  });

  it("updates sensor settings", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "センサー" }));

    const motionSync = screen.getByRole("switch", { name: "Motion Sync" });
    expect(motionSync).toHaveAttribute("data-state", "unchecked");
    await user.click(motionSync);
    expect(motionSync).toHaveAttribute("data-state", "checked");

    await selectOption(user, "押下デバウンス", "1 ms");
    expect(screen.getByLabelText("押下デバウンス")).toHaveTextContent("1 ms");

    await selectOption(user, "リフトオフディスタンス", "1.0 mm");
    expect(screen.getByLabelText("リフトオフディスタンス")).toHaveTextContent("1.0 mm");
  });

  it("edits button mappings and protects the last left click", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "ボタン" }));
    expect(screen.getByText("底面 DIY ボタン")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "左クリック を変更" }));
    const guardDialog = await screen.findByRole("dialog");
    await user.click(within(guardDialog).getByLabelText("動作"));
    await user.click(await screen.findByRole("option", { name: "無効" }));
    expect(within(guardDialog).getByText(/左クリックは1つ以上/)).toBeInTheDocument();
    expect(within(guardDialog).getByRole("button", { name: "保存" })).toBeDisabled();
    await user.click(within(guardDialog).getByRole("button", { name: "キャンセル" }));

    await user.click(screen.getByRole("button", { name: "底面 DIY ボタン を変更" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByLabelText("動作"));
    await user.click(await screen.findByRole("option", { name: "ポーリングレート切替" }));
    await user.click(within(dialog).getByRole("button", { name: "保存" }));
    expect(await screen.findByText("ポーリングレート切替")).toBeInTheDocument();
  });

  it("saves presets and locks presets in passive mode", async () => {
    const user = userEvent.setup();
    render(<App />);
    await enterDemo(user);

    await user.click(screen.getByRole("button", { name: "プロファイル" }));
    expect(await screen.findByText(/FPS 競技用/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "保存" }));
    const saveDialog = await screen.findByRole("dialog");
    await user.type(within(saveDialog).getByLabelText("名前"), "カスタム");
    await user.click(within(saveDialog).getByRole("button", { name: "保存" }));
    expect(await screen.findByText("カスタム")).toBeInTheDocument();

    await selectOption(user, "バックグラウンド取得", "ゲーム中ロック (0-TX)");
    expect(screen.getAllByRole("button", { name: "適用" })[0]).toBeDisabled();

    await selectOption(user, "バックグラウンド取得", "通常");
    expect(screen.getAllByRole("button", { name: "適用" })[0]).not.toBeDisabled();
  });
});
