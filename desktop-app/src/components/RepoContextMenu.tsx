import { useLayoutEffect, useRef } from "react";
import { createPortal } from "react-dom";

interface Props {
  label: string;
  x: number;
  y: number;
  trigger: HTMLButtonElement;
  onDelete: () => void;
  onClose: () => void;
}

export function RepoContextMenu({ label, x, y, trigger, onDelete, onClose }: Props) {
  const menuRef = useRef<HTMLDivElement>(null);
  const deleteRef = useRef<HTMLButtonElement>(null);

  useLayoutEffect(() => {
    const menu = menuRef.current;
    if (!menu) return;
    const rect = menu.getBoundingClientRect();
    menu.style.left = `${Math.max(8, Math.min(x, window.innerWidth - rect.width - 8))}px`;
    menu.style.top = `${Math.max(8, Math.min(y, window.innerHeight - rect.height - 8))}px`;
    deleteRef.current?.focus();
    const dismiss = () => onClose();
    const outside = (event: PointerEvent) => {
      if (!menu.contains(event.target as Node)) onClose();
    };
    document.addEventListener("pointerdown", outside, true);
    window.addEventListener("resize", dismiss);
    window.addEventListener("scroll", dismiss, true);
    return () => {
      document.removeEventListener("pointerdown", outside, true);
      window.removeEventListener("resize", dismiss);
      window.removeEventListener("scroll", dismiss, true);
      if (trigger.isConnected) trigger.focus();
    };
  }, [x, y, trigger, onClose]);

  return createPortal(
    <div
      ref={menuRef}
      role="menu"
      aria-label={`${label} options`}
      style={{ left: x, top: y }}
      className="fixed z-50 min-w-40 rounded-md border border-line bg-panel p-1 shadow-lg"
      onContextMenu={(event) => event.preventDefault()}
      onKeyDown={(event) => {
        if (["Escape", "Tab"].includes(event.key)) {
          event.preventDefault();
          onClose();
        } else if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
          event.preventDefault();
          deleteRef.current?.focus();
        }
      }}
    >
      <button
        ref={deleteRef}
        type="button"
        role="menuitem"
        className="min-h-9 w-full rounded px-3 text-left text-bad hover:bg-selected focus:bg-selected"
        onClick={onDelete}
      >
        Delete
      </button>
    </div>,
    document.body,
  );
}
