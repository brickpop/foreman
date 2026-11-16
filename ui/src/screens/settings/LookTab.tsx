import * as ToggleGroup from "@radix-ui/react-toggle-group";
import { useState } from "react";
import type { Settings as SettingsData } from "../../api/client";
import { permission } from "../../app/notifications";
import { type Look, useLook, useSaveLook } from "../../app/queries";
import { type Accent, accents, asAccent, asTheme } from "../../app/theme";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { terminalFont, terminalFontSizes, terminalFonts } from "../../components/terminalFonts";
import { Button, Loading, Segmented, Switch } from "../../components/ui";
import { onThisMachine, Row, Section, useSave } from "./parts";

/** Settings → Notifications & look. */
export function LookTab({ settings }: { settings?: SettingsData }) {
  return (
    <>
      {settings && <NotificationsSection settings={settings} />}
      <AppearanceSection />
    </>
  );
}

const permissionColor = {
  on: "text-ok",
  blocked: "text-err",
  "turn on": "text-attnTx",
  "n/a": "text-tx4",
};

function NotificationsSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const toast = useToast();
  const [state, setState] = useState(permission);
  const toggles: { key: keyof SettingsData["notifications"]; label: string }[] = [
    { key: "needsMe", label: "An agent needs me" },
    { key: "finishes", label: "An agent finishes" },
    { key: "mergeRequested", label: "A merge is requested" },
  ];
  return (
    <Section label="Notifications" kept={onThisMachine}>
      <Row
        title="Browser notifications"
        explanation="Shown while Foreman isn’t the tab you’re looking at. In-app toasts always show."
      >
        <span className={`text-[13px] ${permissionColor[state]}`}>{state}</span>
        {state === "turn on" && (
          <Button
            variant="small"
            onClick={async () => {
              await Notification.requestPermission();
              setState(permission());
              if (permission() === "on")
                toast({
                  title: "Notifications on",
                  body: "You'll hear from agents when they finish or need you.",
                  color: "var(--ok)",
                });
            }}
          >
            Request
          </Button>
        )}
      </Row>
      {toggles.map((t) => (
        <Row key={t.key} title={t.label}>
          <Switch
            label={t.label}
            checked={settings.notifications[t.key]}
            onChange={(on) =>
              save((s) => ({ ...s, notifications: { ...s.notifications, [t.key]: on } }))
            }
          />
        </Row>
      ))}
    </Section>
  );
}

/** The theme, the accent and the terminal's font: settings of this machine. */
function AppearanceSection() {
  const prefs = useLook();
  const save = useSaveLook();
  const toast = useToast();
  const change = (p: Partial<Look>, what: string) =>
    save(p).catch((e: Error) =>
      toast({ title: `${what} not saved`, body: e.message, color: "var(--err)" }),
    );
  return (
    <Section label="Appearance" kept={onThisMachine}>
      {prefs ? (
        <>
          <Row title="Theme" explanation="System follows your operating system.">
            <Segmented
              label="Theme"
              value={asTheme(prefs.theme)}
              onChange={(theme) => change({ theme }, "Theme")}
              options={[
                { value: "system", label: "System" },
                { value: "dark", label: "Dark" },
                { value: "light", label: "Light" },
              ]}
            />
          </Row>
          <Row
            title="Accent"
            explanation="Buttons, selection and links. What needs you keeps its own amber."
          >
            <AccentPicker
              value={asAccent(prefs.accent)}
              onChange={(accent) => change({ accent }, "Accent")}
            />
          </Row>
          <TerminalFontRow prefs={prefs} onChange={(p) => change(p, "Font")} />
        </>
      ) : (
        <Loading rows={3} />
      )}
    </Section>
  );
}

/** A swatch per accent, each in its own color (styles.css scopes an accent
 * to any element with data-accent). */
function AccentPicker({ value, onChange }: { value: Accent; onChange: (a: Accent) => void }) {
  return (
    <ToggleGroup.Root
      type="single"
      value={value}
      onValueChange={(v) => v && onChange(asAccent(v))}
      aria-label="Accent"
      className="flex gap-1.5"
    >
      {accents.map((a) => (
        <ToggleGroup.Item
          key={a}
          value={a}
          data-accent={a}
          aria-label={accentLabel(a)}
          title={accentLabel(a)}
          className={`grid size-[26px] place-items-center rounded-full border-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-acc ${
            a === value ? "border-acc" : "border-transparent hover:border-line3"
          }`}
        >
          <span className="size-4 rounded-full bg-acc" />
        </ToggleGroup.Item>
      ))}
    </ToggleGroup.Root>
  );
}

const accentLabel = (a: Accent) => a[0].toUpperCase() + a.slice(1);

function TerminalFontRow({
  prefs,
  onChange,
}: {
  prefs: Look;
  onChange: (p: Partial<Look>) => void;
}) {
  return (
    <Row
      title="Terminal font"
      explanation="The agents’ terminals. System uses your operating system’s monospace font."
    >
      <span className="flex gap-2">
        <Select
          label="Terminal font"
          value={terminalFont(prefs.terminalFont).id}
          options={terminalFonts.map((f) => ({ value: f.id, label: f.label }))}
          onChange={(terminalFont) => onChange({ terminalFont })}
        />
        <Select
          label="Terminal font size"
          value={prefs.terminalFontSize}
          options={terminalFontSizes.map((s) => ({ value: s, label: `${s} px` }))}
          onChange={(terminalFontSize) => onChange({ terminalFontSize })}
        />
      </span>
    </Row>
  );
}
