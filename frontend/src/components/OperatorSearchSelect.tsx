import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { sortUsersByPinyin, userDisplayLabel, userMatchesQuery, type UserSortInput } from "../domain/user-sort";
import type { OperatorNameCandidate } from "../domain/audit-operators";

export type OperatorOption = UserSortInput & { id: number };

type OperatorSearchSelectProps = {
  users: OperatorOption[];
  nameCandidates?: OperatorNameCandidate[];
  value: string;
  onChange: (value: string) => void;
};

const ALL_OPTION_ID = "";

export function OperatorSearchSelect({ users, nameCandidates = [], value, onChange }: OperatorSearchSelectProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  const sortedUsers = useMemo(() => sortUsersByPinyin(users), [users]);
  const sortedNames = useMemo(
    () => sortUsersByPinyin(nameCandidates.map((candidate) => ({ ...candidate, displayName: candidate.displayName || candidate.email }))),
    [nameCandidates],
  );
  const selectedUser = sortedUsers.find((user) => String(user.id) === value);
  const selectedName = value.startsWith("name:") || value.startsWith("email:") ? sortedNames.find((candidate) => candidate.key === value) : undefined;
  const options = useMemo(() => {
    const matched = sortedUsers.filter((user) => userMatchesQuery(user, query));
    const matchedNames = sortedNames.filter((candidate) => userMatchesQuery(candidate, query));
    return [
      { id: ALL_OPTION_ID, label: "全部操作人" },
      ...matched.map((user) => ({ id: String(user.id), label: userDisplayLabel(user) })),
      ...matchedNames.map((candidate) => ({ id: candidate.key, label: `${userDisplayLabel(candidate)}（按姓名匹配）` })),
    ];
  }, [sortedUsers, sortedNames, query]);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onPointerDown);
    return () => document.removeEventListener("mousedown", onPointerDown);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    listRef.current?.querySelector(`[data-option-index="${activeIndex}"]`)?.scrollIntoView({ block: "nearest" });
  }, [activeIndex, open]);

  function openPanel() {
    setQuery("");
    const selectedIndex = options.findIndex((option) => option.id === value);
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : 0);
    setOpen(true);
  }

  function selectOption(index: number) {
    const option = options[index];
    if (!option) return;
    onChange(option.id);
    setOpen(false);
    triggerRef.current?.focus();
  }

  function onSearchKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => Math.min(index + 1, options.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(index - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      selectOption(activeIndex);
    } else if (event.key === "Escape") {
      event.preventDefault();
      setOpen(false);
      triggerRef.current?.focus();
    }
  }

  return (
    <div className="operator-select" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="operator-select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        onClick={() => (open ? setOpen(false) : openPanel())}
      >
        <span className="operator-select-value">
          {selectedUser ? userDisplayLabel(selectedUser) : selectedName ? `${userDisplayLabel(selectedName)}（按姓名匹配）` : "全部操作人"}
        </span>
        <span className="operator-select-caret" aria-hidden="true">▾</span>
      </button>
      {open ? (
        <div className="operator-select-panel">
          <input
            ref={inputRef}
            type="text"
            className="operator-select-search"
            role="combobox"
            aria-expanded="true"
            aria-controls={listId}
            aria-activedescendant={options[activeIndex] ? `${listId}-option-${activeIndex}` : undefined}
            aria-label="搜索操作人"
            placeholder="搜索姓名 / 账号 / 邮箱"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={onSearchKeyDown}
          />
          <ul className="operator-select-list" id={listId} role="listbox" aria-label="操作人列表">
            {options.map((option, index) => (
              <li
                key={option.id || "all"}
                id={`${listId}-option-${index}`}
                role="option"
                aria-selected={option.id === value}
                data-option-index={index}
                className={`operator-select-option${index === activeIndex ? " is-active" : ""}${option.id === value ? " is-selected" : ""}`}
                onMouseEnter={() => setActiveIndex(index)}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => selectOption(index)}
              >
                {option.label}
              </li>
            ))}
            {query.trim() && options.length === 1 ? (
              <li className="operator-select-empty" role="presentation">无匹配操作人</li>
            ) : null}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
