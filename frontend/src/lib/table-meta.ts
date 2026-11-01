import type { RowData } from '@tanstack/react-table';

// Row actions reach the module-level column definitions through table meta,
// so cell renderers never close over component state.
declare module '@tanstack/react-table' {
  // The generic parameter must match TanStack's declaration to merge.
  interface TableMeta<TData extends RowData> {
    onOpen?: (id: string) => void;
    onDelete?: (id: string) => void;
    /** Reference time for relative dates, taken when the rows were fetched. */
    now?: number;
  }
}
