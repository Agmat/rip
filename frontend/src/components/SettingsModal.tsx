"use client";

import { useRef } from "react";
import type { Settings } from "@/lib/settings";

// Native <dialog>: showModal() gives focus trapping, Esc to close and the
// backdrop for free.
export default function SettingsModal({
  settings,
  onChange,
}: {
  settings: Settings;
  onChange: (s: Settings) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);

  return (
    <>
      <button onClick={() => dialog.current?.showModal()} className="ghost-button">
        settings
      </button>
      <dialog
        ref={dialog}
        // Click on the backdrop (the dialog itself, outside the panel) closes it.
        onClick={(e) => e.target === dialog.current && dialog.current.close()}
        className="settings-dialog"
      >
        <div className="flex flex-col gap-4 p-5">
          <h2 className="display text-lg">Settings</h2>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.dimEnabled}
              onChange={(e) => onChange({ ...settings, dimEnabled: e.target.checked })}
            />
            Dim cards worth less than
          </label>
          <label className="flex items-center gap-2 text-sm text-muted">
            €
            <input
              type="number"
              min={0}
              step={0.01}
              defaultValue={settings.dimBelow}
              disabled={!settings.dimEnabled}
              aria-label="Dim threshold in euros"
              // Uncontrolled so the field can be cleared mid-edit; only valid numbers are saved.
              onChange={(e) => {
                const v = e.target.valueAsNumber;
                if (Number.isFinite(v) && v >= 0) onChange({ ...settings, dimBelow: v });
              }}
              className="w-24 rounded bg-bg px-2 py-1 text-ink disabled:opacity-50"
            />
          </label>
          <form method="dialog" className="flex justify-end">
            <button className="ghost-button">done</button>
          </form>
        </div>
      </dialog>
    </>
  );
}
