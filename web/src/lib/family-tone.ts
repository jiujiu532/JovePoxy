/** Deterministic accent chip for model family monograms. */

export type FamilyTone = {
  readonly bg: string;
  readonly text: string;
};

const TONES: readonly FamilyTone[] = [
  { bg: "bg-accent-yellow", text: "text-black" },
  { bg: "bg-accent-teal", text: "text-black" },
  { bg: "bg-accent-mint", text: "text-black" },
  { bg: "bg-accent-soft", text: "text-ink" },
  { bg: "bg-accent-coral", text: "text-black" },
  { bg: "bg-accent", text: "text-black" },
] as const;

function hash(input: string): number {
  let h = 0;
  for (let i = 0; i < input.length; i += 1) {
    h = (h * 31 + input.charCodeAt(i)) | 0;
  }
  return Math.abs(h);
}

export function familyTone(family: string): FamilyTone {
  return TONES[hash(family.toLowerCase()) % TONES.length] ?? TONES[0]!;
}

export function familyInitials(family: string): string {
  const clean = family.trim();
  if (!clean) return "?";
  const parts = clean.split(/[-_\s]+/).filter(Boolean);
  if (parts.length >= 2) {
    return `${parts[0]![0] ?? ""}${parts[1]![0] ?? ""}`.toUpperCase();
  }
  return clean.slice(0, 2).toUpperCase();
}
