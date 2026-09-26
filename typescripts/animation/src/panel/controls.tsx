import { Select as SelectPrimitive } from '@base-ui/react/select';
import { useId } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { RadioGroup, RadioGroupItem } from './ui/radio-group';
import { Select as SelectRoot, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Slider as SliderRoot } from './ui/slider';
import { Switch } from './ui/switch';

// Building blocks for the control panel, built from the shadcn/ui components
// in ./ui (Base UI underneath). Each shows the value it is given and reports a
// change through onChange; the value itself lives in the settings store.

// A row of the panel's grid (label, control, value), so labels and controls
// line up however long a label is. `group` with data-disabled greys out the
// label and the value along with the control.
const row = 'group col-span-full grid grid-cols-subgrid items-center';

// On or off, shown as a switch; clicking the label flips it too.
export function Toggle({
  label,
  on,
  disabled = false,
  onChange,
}: {
  label: string;
  on: boolean;
  disabled?: boolean; // greyed out
  onChange: (on: boolean) => void;
}) {
  return (
    <Label className={`${row} cursor-pointer data-[disabled=true]:cursor-default`} data-disabled={disabled}>
      <span className="group-data-[disabled=true]:opacity-50">{label}</span>
      <Switch checked={on} disabled={disabled} onCheckedChange={(checked) => onChange(checked)} />
    </Label>
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
  const labelId = useId();
  return (
    <div className={row} data-disabled={disabled}>
      <Label id={labelId}>{label}</Label>
      {/* One thumb: the shadcn slider draws a thumb per value, so the value
          goes in as a one-element list. */}
      <SliderRoot
        aria-labelledby={labelId}
        value={[value]}
        min={min}
        max={max}
        step={step}
        disabled={disabled}
        onValueChange={(v) => onChange(typeof v === 'number' ? v : v[0])}
      />
      <output className="min-w-[3.5em] text-right tabular-nums group-data-[disabled=true]:opacity-50">
        {unit ? `${value} ${unit}` : value}
      </output>
    </div>
  );
}

export interface Option {
  value: string;
  label: string;
  hint?: string; // a note shown after the label in a RadioList
}

// A few options, one row each with a radio button; clicking a row picks it.
export function RadioList({
  label,
  value,
  options,
  disabled = false,
  onChange,
}: {
  label: string; // for screen readers
  value: string;
  options: Option[];
  disabled?: boolean; // greyed out
  onChange: (value: string) => void;
}) {
  return (
    <RadioGroup aria-label={label} value={value} disabled={disabled} onValueChange={onChange} className="gap-0.5">
      {options.map((option) => (
        <Label
          key={option.value}
          className="h-8 cursor-pointer rounded-md px-2 font-normal hover:bg-accent has-data-checked:bg-accent has-data-checked:font-medium has-data-disabled:pointer-events-none has-data-disabled:opacity-50"
        >
          <RadioGroupItem value={option.value} />
          {option.label}
          {option.hint && <span className="ml-auto text-xs font-normal text-muted-foreground">{option.hint}</span>}
        </Label>
      ))}
    </RadioGroup>
  );
}

// A menu with its label. The menu spans the control and value columns.
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
    <div className={row} data-disabled={disabled}>
      {/* items lets the menu show an option's label rather than its value. */}
      <SelectRoot items={options} value={value} disabled={disabled} onValueChange={(v) => onChange(v ?? '')}>
        {/* Base UI's own label: it names the menu, and a click on it focuses
            the menu without opening it. */}
        <SelectPrimitive.Label render={<Label />}>{label}</SelectPrimitive.Label>
        <SelectTrigger className="col-span-2 w-full min-w-0">
          <SelectValue />
        </SelectTrigger>
        {/* The list drops below the menu. Laid over it with the picked option
            on the menu (the default), the options above the picked one went
            under a scroll arrow at the top of the screen, where the panel is. */}
        <SelectContent alignItemWithTrigger={false}>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </SelectRoot>
    </div>
  );
}

// A button that does something once, such as running a figure's behaviour.
// `wide` spans the whole panel, outlined unless it is the tab's main action
// (`tone` main, such as Go live); `tone` danger is one that ends something
// (Stop streaming).
export function Action({
  label,
  wide = false,
  tone,
  disabled = false,
  onClick,
}: {
  label: string;
  wide?: boolean;
  tone?: 'main' | 'danger';
  disabled?: boolean;
  onClick: () => void;
}) {
  const variant = tone === 'danger' ? 'destructive' : wide && tone !== 'main' ? 'outline' : 'default';
  return (
    <Button variant={variant} className={wide ? 'col-span-full' : undefined} disabled={disabled} onClick={onClick}>
      {label}
    </Button>
  );
}

// A text field with its label, spanning the control and value columns.
// `secret` hides what is typed (a stream key) and keeps the browser from
// offering to save it.
export function TextField({
  label,
  value,
  placeholder,
  secret = false,
  disabled = false,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  secret?: boolean;
  disabled?: boolean; // greyed out
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className={row} data-disabled={disabled}>
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        className="col-span-2 min-w-0"
        type={secret ? 'password' : 'text'}
        autoComplete={secret ? 'new-password' : 'off'}
        spellCheck={false}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onChange={(e) => onChange(e.currentTarget.value)}
      />
    </div>
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
    <Input
      className="flex-1"
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
    <SelectRoot value={value} onValueChange={(v) => onChange(v ?? '')}>
      <SelectTrigger className="min-w-0 flex-1" aria-label={name}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {options.map((option) => (
          <SelectItem key={option} value={option}>
            {option}
          </SelectItem>
        ))}
      </SelectContent>
    </SelectRoot>
  );
}
