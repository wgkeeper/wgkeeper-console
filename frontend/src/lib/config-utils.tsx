const renderConfigLine = (line: string) => {
  const trimmed = line.trim();
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    return <span className="font-semibold text-foreground">{line}</span>;
  }
  if (trimmed.startsWith('#') || trimmed.startsWith(';')) {
    return <span className="text-muted-foreground italic">{line}</span>;
  }
  const eqIndex = line.indexOf('=');
  if (eqIndex !== -1) {
    const left = line.slice(0, eqIndex);
    const right = line.slice(eqIndex + 1);
    return (
      <span>
        <span className="text-sky-700 dark:text-foreground/90">{left}</span>
        <span className="text-muted-foreground">=</span>
        <span className="text-foreground/65 dark:text-muted-foreground">{right}</span>
      </span>
    );
  }
  return <span>{line}</span>;
};

export const renderConfig = (text: string) =>
  text.split('\n').map((line, index, arr) => (
    <span key={index}>
      {renderConfigLine(line)}
      {index < arr.length - 1 ? '\n' : null}
    </span>
  ));
