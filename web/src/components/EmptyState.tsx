interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description: string;
  action?: React.ReactNode;
  as?: 'h2' | 'h3';
}

export function EmptyState({ icon, title, description, action, as: Heading = 'h2' }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-[var(--color-text-tertiary)] animate-fade-in">
      {icon && <div className="text-4xl mb-4">{icon}</div>}
      <Heading className="text-lg font-medium text-foreground">{title}</Heading>
      <p className="mt-1 text-sm">{description}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
