interface SectionHeadingProps {
  children: React.ReactNode;
  as?: 'h2' | 'h3';
}

export function SectionHeading({ children, as: Tag = 'h3' }: SectionHeadingProps) {
  return (
    <Tag className="text-[var(--text-heading-2)] font-semibold text-foreground mb-4 pl-3 border-l-2 border-[var(--accent-provider)]">
      {children}
    </Tag>
  );
}
