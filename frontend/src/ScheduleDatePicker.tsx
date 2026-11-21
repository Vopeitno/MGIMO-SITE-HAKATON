import {useId, useLayoutEffect, useRef, useState} from 'react';
import type {KeyboardEvent, ReactNode} from 'react';
import {shiftDate, weekPeriod} from './schedule-model';
import {moscowDate} from './today-model';
import './schedule-date-picker.css';

const fullDate = new Intl.DateTimeFormat('ru-RU', {day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC'});
const monthLabel = new Intl.DateTimeFormat('ru-RU', {month: 'long', year: 'numeric', timeZone: 'UTC'});
const weekdays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

function calendarDate(value: string) {
  return new Date(value + 'T12:00:00Z');
}

function shiftMonth(value: string, step: number) {
  const month = calendarDate(value.slice(0, 7) + '-01');
  month.setUTCMonth(month.getUTCMonth() + step);
  const lastDay = new Date(month);
  lastDay.setUTCMonth(lastDay.getUTCMonth() + 1, 0);
  month.setUTCDate(Math.min(calendarDate(value).getUTCDate(), lastDay.getUTCDate()));
  return month.toISOString().slice(0, 10);
}

export function ScheduleDatePicker({value, onChange, icon}: {value: string; onChange: (value: string) => void; icon: ReactNode}) {
  const [open, setOpen] = useState(false);
  const [cursor, setCursor] = useState(value);
  const dialog = useRef<HTMLDialogElement>(null);
  const activeDay = useRef<HTMLButtonElement>(null);
  const heading = useId();
  const calendar = useId();
  const focusDay = useRef(false);
  const today = moscowDate();
  const month = cursor.slice(0, 7);
  const start = weekPeriod(month + '-01').from;
  const weeks = Array.from({length: 6}, (_, row) => shiftDate(start, row * 7));

  useLayoutEffect(() => {
    if (!open) return;
    if (!dialog.current?.open) dialog.current?.showModal();
    if (focusDay.current) {
      activeDay.current?.focus();
      focusDay.current = false;
    }
  }, [open, cursor]);

  function select(date: string) {
    onChange(date);
    dialog.current?.close();
  }

  function navigate(event: KeyboardEvent<HTMLButtonElement>, date: string) {
    const week = weekPeriod(date);
    const dates: Record<string, string> = {
      ArrowLeft: shiftDate(date, -1),
      ArrowRight: shiftDate(date, 1),
      ArrowUp: shiftDate(date, -7),
      ArrowDown: shiftDate(date, 7),
      Home: week.from,
      End: week.to,
      PageUp: shiftMonth(date, -1),
      PageDown: shiftMonth(date, 1),
    };
    const next = dates[event.key];
    if (!next || next === date) return;
    event.preventDefault();
    focusDay.current = true;
    setCursor(next);
  }

  return <>
    <button type="button" className="secondary schedule-date-trigger" aria-haspopup="dialog" aria-expanded={open} aria-controls={calendar}
      onClick={() => {focusDay.current = true; setCursor(value); setOpen(true);}}>
      {icon}<time dateTime={value}>{fullDate.format(calendarDate(value))}</time>
    </button>
    <dialog ref={dialog} className="modal schedule-date-dialog" id={calendar} aria-label="Выбор даты расписания" onClose={() => setOpen(false)}>
      <div className="schedule-date-header">
        <h2 className="card-title">Выбор даты</h2>
        <button type="button" className="icon-btn" aria-label="Закрыть календарь" onClick={() => dialog.current?.close()}>×</button>
      </div>
      <div className="schedule-date-month">
        <button type="button" className="filter" aria-label="Предыдущий месяц" onClick={() => setCursor(shiftMonth(cursor, -1))}>←</button>
        <h3 id={heading} aria-live="polite">{monthLabel.format(calendarDate(cursor))}</h3>
        <button type="button" className="filter" aria-label="Следующий месяц" onClick={() => setCursor(shiftMonth(cursor, 1))}>→</button>
      </div>
      <table className="schedule-date-grid" role="grid" aria-labelledby={heading}>
        <thead><tr>{weekdays.map(day => <th key={day} scope="col">{day}</th>)}</tr></thead>
        <tbody>{weeks.map(week => <tr key={week} data-outside-month={week.slice(0, 7) > month}>{weekdays.map((day, index) => {
          const date = shiftDate(week, index);
          return <td key={day} aria-selected={value === date}>
            <button type="button" className="schedule-date-day" ref={date === cursor ? activeDay : null}
              tabIndex={date === cursor ? 0 : -1} aria-label={fullDate.format(calendarDate(date))}
              aria-current={date === today ? 'date' : undefined} data-adjacent={date.slice(0, 7) !== month}
              onKeyDown={event => navigate(event, date)} onClick={() => select(date)}>
              {Number(date.slice(-2))}
            </button>
          </td>;
        })}</tr>)}</tbody>
      </table>
      <div className="modal-actions">
        <button type="button" className="secondary" onClick={() => select(today)}>Сегодня</button>
      </div>
    </dialog>
  </>;
}
