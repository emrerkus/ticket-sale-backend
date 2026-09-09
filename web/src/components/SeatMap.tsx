import type { SeatMapEntry } from "../types";

interface Props {
  seats: SeatMapEntry[];
  selectedIds: Set<string>;
  busyId: string | null;
  onToggle: (seat: SeatMapEntry) => void;
}

/** Koltuklari bolum -> sira duzeninde bir izgara olarak cizer. */
export function SeatMap({ seats, selectedIds, busyId, onToggle }: Props) {
  const sections = groupBy(seats, (s) => s.section);

  return (
    <div className="seatmap">
      {Object.entries(sections).map(([section, secSeats]) => {
        const rows = groupBy(secSeats, (s) => s.row_label);
        return (
          <div key={section} className="section">
            <h4>{section} Blok</h4>
            {Object.entries(rows)
              .sort((a, b) => Number(a[0]) - Number(b[0]))
              .map(([row, rowSeats]) => (
                <div key={row} className="seatrow">
                  <span className="rowlabel">{row}</span>
                  {rowSeats
                    .slice()
                    .sort((a, b) => Number(a.seat_number) - Number(b.seat_number))
                    .map((seat) => {
                      const mine = selectedIds.has(seat.seat_id);
                      const cls = mine
                        ? "seat mine"
                        : `seat ${seat.status}`;
                      const disabled =
                        busyId === seat.seat_id ||
                        (seat.status !== "available" && !mine);
                      return (
                        <button
                          key={seat.seat_id}
                          className={cls}
                          disabled={disabled}
                          title={`${seat.section}${seat.row_label}-${seat.seat_number} · ${labelFor(seat, mine)}`}
                          onClick={() => onToggle(seat)}
                        >
                          {seat.seat_number}
                        </button>
                      );
                    })}
                </div>
              ))}
          </div>
        );
      })}
      <div className="legend">
        <span><i className="swatch available" /> bos</span>
        <span><i className="swatch mine" /> senin secimin</span>
        <span><i className="swatch held" /> baskasi tutuyor</span>
        <span><i className="swatch sold" /> satildi</span>
      </div>
    </div>
  );
}

function labelFor(s: SeatMapEntry, mine: boolean): string {
  if (mine) return "senin secimin";
  return { available: "bos", held: "tutuluyor", sold: "satildi" }[s.status];
}

function groupBy<T>(arr: T[], key: (t: T) => string): Record<string, T[]> {
  return arr.reduce<Record<string, T[]>>((acc, item) => {
    (acc[key(item)] ??= []).push(item);
    return acc;
  }, {});
}
