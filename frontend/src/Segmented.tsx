type Option<T extends string | number> = { value: T; label: string; title?: string };

/**
 * A compact single-choice control for filters inside a panel: a range of
 * history, a grouping. Unlike the Manifest mode cards it sits inline with a
 * panel title and never wraps.
 */
export function Segmented<T extends string | number>({ label, options, value, onChange }: { label: string; options: Option<T>[]; value: T; onChange: (value: T) => void }) {
  return (
    <div className="segmented" role="radiogroup" aria-label={label}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={option.value === value}
          className={`segmented-option ${option.value === value ? "active" : ""}`}
          title={option.title}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
