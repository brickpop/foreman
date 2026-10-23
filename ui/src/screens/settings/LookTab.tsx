import { useState } from "react";
import type { Preferences, Settings as SettingsData } from "../../api/client";
import { permission } from "../../app/notifications";
import { usePreferences, useSaveTerminalFont, useSaveTheme } from "../../app/queries";
import { asTheme, savedTheme } from "../../app/theme";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { terminalFont, terminalFontSizes, terminalFonts } from "../../components/terminalFonts";
import { Button, Segmented, Switch } from "../../components/ui";
import { inGlobalRepo, onThisMachine, Row, Section, useSave } from "./parts";

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
  "turn on": "text-accTx",
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
    <Section label="Notifications" kept={inGlobalRepo}>
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

/** The theme, one of the CEO's preferences. */
function AppearanceSection() {
  const prefs = usePreferences();
  const save = useSaveTheme();
  const toast = useToast();
  const theme =
    save.isPending && save.variables
      ? save.variables
      : prefs.data
        ? asTheme(prefs.data.theme)
        : savedTheme();
  return (
    <Section label="Appearance" kept={onThisMachine}>
      <Row title="Theme" explanation="System follows your operating system.">
        <Segmented
          label="Theme"
          value={theme}
          onChange={(t) =>
            save.mutate(t, {
              onError: (e) =>
                toast({ title: "Theme not saved", body: e.message, color: "var(--err)" }),
            })
          }
          options={[
            { value: "system", label: "System" },
            { value: "dark", label: "Dark" },
            { value: "light", label: "Light" },
          ]}
        />
      </Row>
      {prefs.data && <TerminalFontRow prefs={prefs.data} />}
    </Section>
  );
}

function TerminalFontRow({ prefs }: { prefs: Preferences }) {
  const save = useSaveTerminalFont();
  const toast = useToast();
  const change = (p: Partial<Preferences>) =>
    save.mutate(p, {
      onError: (e) => toast({ title: "Font not saved", body: e.message, color: "var(--err)" }),
    });
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
          onChange={(terminalFont) => change({ terminalFont })}
        />
        <Select
          label="Terminal font size"
          value={prefs.terminalFontSize}
          options={terminalFontSizes.map((s) => ({ value: s, label: `${s} px` }))}
          onChange={(terminalFontSize) => change({ terminalFontSize })}
        />
      </span>
    </Row>
  );
}
