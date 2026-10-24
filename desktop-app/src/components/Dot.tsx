export function Dot({ color }: { color: string }) {
  return <span className="inline-block size-1.5 shrink-0 rounded-full" style={{ background: color }} aria-hidden="true" />;
}
