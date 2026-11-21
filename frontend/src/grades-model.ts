export type GradeCourse = { courseID: string; title: string; semester: number; teachers?: { teacherID: string; teacherName: string }[] };
export type CourseGrade = { courseGraduate: string; courseID: string; score: number | null; credit: boolean | null };

export function buildGradesModel(courses: GradeCourse[], grades: CourseGrade[]) {
  const byCourse = new Map(grades.map(g => [g.courseID, g]));
  const subjects = courses.map(course => {
    const grade = byCourse.get(course.courseID);
    const score = typeof grade?.score === 'number' && Number.isFinite(grade.score) && grade.score >= 0 && grade.score <= 100 ? grade.score : null;
    const credit = score === null ? null : grade?.credit ?? null;
    return {
      id: course.courseID, n: course.title, t: course.teachers?.map(t => t.teacherName).filter(Boolean).join(', ') || 'Преподаватель не назначен',
      s: score, credit, semester: course.semester,
      status: credit === true ? 'Зачтено' : credit === false ? 'Незачёт' : 'Нет оценки',
      mark: credit === true ? '✓' : credit === false ? '×' : '—',
    };
  });
  return summarizeGrades(subjects);
}

type GradeSubject = {
  id: string; n: string; t: string; s: number | null; credit: boolean | null;
  semester: number; status: string; mark: string;
};

function summarizeGrades(subjects: GradeSubject[]) {
  const scored = subjects.flatMap(subject => subject.s === null ? [] : [subject.s]);
  const average = scored.length ? scored.reduce((sum, score) => sum + score, 0) / scored.length : null;
  return {
    subjects,
    average: average === null ? '—' : new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(average),
    completed: subjects.filter(s => s.credit === true).length,
    total: subjects.length,
  };
}

export function gradesForYear(model: GradesModel, year: string): GradesModel {
  if (year === 'all') return model;
  const selected = Number(year);
  return summarizeGrades(model.subjects.filter(subject =>
    Number.isInteger(subject.semester) && subject.semester > 0 && Math.ceil(subject.semester / 2) === selected));
}
export type GradesModel = ReturnType<typeof buildGradesModel>;

