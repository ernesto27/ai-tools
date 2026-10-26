interface Props {
  message: string;
}

export function StatusMessage({ message }: Props) {
  return (
    <div role="status" className="flex items-start gap-3 rounded-md border border-ok/30 bg-ok/10 px-4 py-3 text-sm text-ok">
      <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="mt-0.5 size-4 shrink-0">
        <path d="m5 12 4 4L19 6" />
      </svg>
      <p className="min-w-0 break-words">{message}</p>
    </div>
  );
}
