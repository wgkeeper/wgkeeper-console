type Props = {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  children?: React.ReactNode;
};

/** Title row of a page: heading and description on the left, actions on the right. */
export const PageHeader = ({ title, description, actions, children }: Props) => (
  <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
    <div className="flex min-w-0 flex-col gap-1">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      {description === undefined ? null : (
        <p className="text-sm text-muted-foreground">{description}</p>
      )}
      {children}
    </div>
    {actions === undefined ? null : (
      <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
    )}
  </div>
);
