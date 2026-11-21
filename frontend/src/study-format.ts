const dateTime = new Intl.DateTimeFormat('ru-RU', {dateStyle: 'short', timeStyle: 'short', timeZone: 'Europe/Moscow'});
export const studyDate = (date: string) => dateTime.format(new Date(date));
export function studyError(error: unknown) {
    return error instanceof Error ? error.message : 'Запрос не выполнен. Попробуйте ещё раз';
}

