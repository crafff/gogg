import { forwardRef } from "react";
import * as SelectPrimitive from "@radix-ui/react-select";

import { cn } from "@shared/lib/cn";

// Thin Radix wrapper — exposes only what chunk 2 needs (Root, Trigger,
// Value, Content, Item) so consumers don't reach into the primitive
// directly. The full surface (Group, Label, Separator) can be added
// when a real form needs it.

export const Select = SelectPrimitive.Root;
export const SelectValue = SelectPrimitive.Value;

interface TriggerProps extends React.ComponentPropsWithoutRef<
  typeof SelectPrimitive.Trigger
> {
  size?: "sm" | "md";
}

export const SelectTrigger = forwardRef<
  React.ElementRef<typeof SelectPrimitive.Trigger>,
  TriggerProps
>(({ className, size = "md", children, ...props }, ref) => (
  <SelectPrimitive.Trigger
    ref={ref}
    className={cn(
      "inline-flex items-center justify-between gap-2 rounded border " +
        "border-border-default bg-surface-raised text-fg-default " +
        "hover:border-border-strong " +
        "focus-visible:outline-none focus-visible:shadow-focus-ring " +
        "disabled:cursor-not-allowed disabled:opacity-50",
      size === "sm" ? "h-7 px-2 text-xs" : "h-9 px-3 text-sm",
      className,
    )}
    {...props}
  >
    {children}
    <SelectPrimitive.Icon className="text-fg-subtle">▾</SelectPrimitive.Icon>
  </SelectPrimitive.Trigger>
));
SelectTrigger.displayName = "SelectTrigger";

export const SelectContent = forwardRef<
  React.ElementRef<typeof SelectPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>
>(({ className, children, position = "popper", ...props }, ref) => (
  <SelectPrimitive.Portal>
    <SelectPrimitive.Content
      ref={ref}
      position={position}
      className={cn(
        "z-50 min-w-[var(--radix-select-trigger-width)] overflow-hidden " +
          "rounded border border-border-default bg-surface-overlay shadow-card",
        position === "popper" && "mt-1",
        className,
      )}
      {...props}
    >
      <SelectPrimitive.Viewport className="p-1">
        {children}
      </SelectPrimitive.Viewport>
    </SelectPrimitive.Content>
  </SelectPrimitive.Portal>
));
SelectContent.displayName = "SelectContent";

export const SelectItem = forwardRef<
  React.ElementRef<typeof SelectPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item>
>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Item
    ref={ref}
    className={cn(
      "relative flex cursor-pointer select-none items-center rounded px-2 py-1.5 " +
        "text-sm text-fg-default outline-none " +
        "data-[highlighted]:bg-accent-subtle data-[highlighted]:text-accent " +
        "data-[state=checked]:text-accent " +
        "data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50",
      className,
    )}
    {...props}
  >
    <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
  </SelectPrimitive.Item>
));
SelectItem.displayName = "SelectItem";
