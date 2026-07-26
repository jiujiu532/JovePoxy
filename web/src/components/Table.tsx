import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type TableColumn<T> = {
  readonly id: string;
  readonly header: string;
  readonly cell: (row: T) => ReactNode;
  readonly className?: string;
};

export type TableProps<T> = {
  readonly columns: readonly TableColumn<T>[];
  readonly rows: readonly T[];
  readonly rowKey: (row: T) => string;
  readonly empty?: ReactNode;
  readonly className?: string;
};

export function Table<T>({
  columns,
  rows,
  rowKey,
  empty,
  className,
}: TableProps<T>) {
  if (rows.length === 0 && empty) {
    return <div className={className}>{empty}</div>;
  }

  return (
    <div className={cn("w-full min-w-0", className)}>
      {/* Mobile: stacked field cards */}
      <div className="divide-y-2 divide-border md:hidden">
        {rows.map((row) => {
          const [primary, ...rest] = columns;
          return (
            <div key={rowKey(row)} className="px-3 py-3">
              {primary ? (
                <div className="text-[13px] font-semibold text-ink">
                  {primary.cell(row)}
                </div>
              ) : null}
              {rest.length > 0 ? (
                <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5">
                  {rest.map((col) => (
                    <div key={col.id} className="min-w-0">
                      <dt className="text-[10px] font-medium uppercase tracking-wide text-ink-faint">
                        {col.header}
                      </dt>
                      <dd className="mt-0.5 break-words text-[12px] text-ink-muted">
                        {col.cell(row)}
                      </dd>
                    </div>
                  ))}
                </dl>
              ) : null}
            </div>
          );
        })}
      </div>

      {/* Desktop: table */}
      <div className="hidden overflow-x-auto md:block">
        <table className="w-full min-w-[32rem] border-collapse border-2 border-border text-left text-sm">
          <thead>
            <tr className="border-b-2 border-border bg-paper-0">
              {columns.map((col) => (
                <th
                  key={col.id}
                  scope="col"
                  className={cn(
                    "whitespace-nowrap px-3 py-2 text-caption font-medium text-ink",
                    col.className,
                  )}
                >
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={rowKey(row)}
                className="border-b border-2 border-border last:border-b-0 transition-colors hover:bg-accent-soft"
              >
                {columns.map((col) => (
                  <td
                    key={col.id}
                    className={cn(
                      "h-11 whitespace-nowrap px-3 py-2 align-middle text-inherit",
                      col.className,
                    )}
                  >
                    {col.cell(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
