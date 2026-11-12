import { useEffect, useState } from "react";
import type { KeyboardEvent } from "react";

interface Options {
  id: string;
  optionCount: number;
  disabled: boolean;
  onSelect: (index: number) => void;
  onOpen?: () => void;
  onClose?: () => void;
}

export function useListboxNavigation({ id, optionCount, disabled, onSelect, onOpen, onClose }: Options) {
  const [open, setOpen] = useState(false);
  const [openAbove, setOpenAbove] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const expanded = open && !disabled;
  const listId = `${id}-suggestions`;
  const activeId = expanded && activeIndex >= 0 && activeIndex < optionCount
    ? `${listId}-${activeIndex}` : undefined;

  useEffect(() => {
    if (activeId) document.getElementById(activeId)?.scrollIntoView({ block: "nearest" });
  }, [activeId]);

  function resetHighlight() {
    setActiveIndex(-1);
  }

  function show(input: HTMLInputElement) {
    if (disabled) return;
    const bounds = input.getBoundingClientRect();
    const below = window.innerHeight - bounds.bottom;
    setOpenAbove(below < 208 && bounds.top > below);
    if (!expanded) onOpen?.();
    setOpen(true);
  }

  function close() {
    setOpen(false);
    resetHighlight();
    onClose?.();
  }

  function select(index: number) {
    if (disabled || index < 0 || index >= optionCount) return;
    onSelect(index);
    close();
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (disabled || event.nativeEvent.isComposing) return;
    if (event.key === "Escape" || event.key === "Tab") {
      if (event.key === "Escape" && expanded) event.preventDefault();
      close();
    } else if ((event.key === "ArrowDown" || event.key === "ArrowUp") && optionCount > 0) {
      event.preventDefault();
      show(event.currentTarget);
      setActiveIndex((current) => {
        if (!expanded || current < 0 || current >= optionCount) {
          return event.key === "ArrowDown" ? 0 : optionCount - 1;
        }
        return (current + (event.key === "ArrowDown" ? 1 : -1) + optionCount) % optionCount;
      });
    } else if (event.key === "Enter" && activeId) {
      event.preventDefault();
      select(activeIndex);
    }
  }

  return { expanded, openAbove, activeIndex, activeId, listId, show, close, select, resetHighlight, onKeyDown };
}
