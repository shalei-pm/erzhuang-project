import { useEffect, useRef } from "react";

export function PlaybackSpeedSheet({ onSelect, onClose }: {
  onSelect: (rate: number) => void;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    dialog.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => { if (previous?.isConnected) previous.focus(); };
  }, []);
  return (
    <div className="h5-speed-backdrop" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <div ref={dialog} role="dialog" aria-modal="true" aria-label="选择播放倍速" className="h5-speed-sheet"
        onKeyDown={(event) => {
          if (event.key === "Escape") { event.stopPropagation(); onClose(); }
          if (event.key !== "Tab") return;
          const buttons = dialog.current?.querySelectorAll<HTMLButtonElement>("button");
          if (!buttons?.length) return;
          const first = buttons[0];
          const last = buttons[buttons.length - 1];
          if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
          if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
        }}>
        <div className="h5-speed-sheet-title">播放倍速</div>
        {[8].map((rate) => (
          <button type="button" key={rate} onClick={() => onSelect(rate)}>{rate}倍速</button>
        ))}
        <button type="button" className="h5-speed-cancel" onClick={onClose}>取消</button>
      </div>
    </div>
  );
}
