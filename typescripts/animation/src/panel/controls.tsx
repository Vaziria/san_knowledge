// Building blocks for the control panel, styled in panel.css. Each shows the
// value it is given and reports a change through onChange; the value itself
// lives in the settings store.

export function Toggle({ label, on, onChange }: { label: string; on: boolean; onChange: (on: boolean) => void }) {
  return (
    <button type="button" className="toggle" aria-pressed={on} onClick={() => onChange(!on)}>
      {`${label}: ${on ? 'on' : 'off'}`}
    </button>
  );
}

export function Slider({
  label,
  value,
  min,
  max,
  step = 1,
  unit,
  disabled = false,
  onChange,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  unit?: string;
  disabled?: boolean; // greyed out
  onChange: (value: number) => void;
}) {
  return (
    <label className="slider">
      <span>{label}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(Number(e.currentTarget.value))}
      />
      <output>{unit ? `${value} ${unit}` : value}</output>
    </label>
  );
}

export interface Option {
  value: string;
  label: string;
}

export function Select({
  label,
  value,
  options,
  disabled = false,
  onChange,
}: {
  label: string;
  value: string;
  options: Option[];
  disabled?: boolean; // greyed out
  onChange: (value: string) => void;
}) {
  return (
    <label className="select">
      <span>{label}</span>
      <select value={value} disabled={disabled} onChange={(e) => onChange(e.currentTarget.value)}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

// A button that does something once, such as running a figure's behaviour.
// `wide` spans the whole panel.
export function Action({ label, wide = false, onClick }: { label: string; wide?: boolean; onClick: () => void }) {
  return (
    <button type="button" className={wide ? 'action wide' : 'action'} onClick={onClick}>
      {label}
    </button>
  );
}

// A bare text field (its label is the button beside it). Enter submits.
export function Field({
  name,
  value,
  onChange,
  onSubmit,
}: {
  name: string; // for screen readers
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
}) {
  return (
    <input
      type="text"
      className="field"
      aria-label={name}
      value={value}
      onChange={(e) => onChange(e.currentTarget.value)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') onSubmit();
      }}
    />
  );
}

// A bare menu (its label is the button beside it).
export function Choice({
  name,
  value,
  options,
  onChange,
}: {
  name: string; // for screen readers
  value: string;
  options: string[];
  onChange: (value: string) => void;
}) {
  return (
    <select className="choice" aria-label={name} value={value} onChange={(e) => onChange(e.currentTarget.value)}>
      {options.map((option) => (
        <option key={option} value={option}>
          {option}
        </option>
      ))}
    </select>
  );
}
