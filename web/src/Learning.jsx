import { BookOpen, Check, Clock3, GraduationCap, MessageCircle, NotebookPen } from 'lucide-react'
import { useState } from 'react'
import { levels, languages } from './lib'
import { ErrorMessage } from './Dialogs'

export function Program({ school, onModule, onBook }) {
  const [language, setLanguage] = useState('english')
  const books = school?.books.filter(book => book.language === language) || []
  return (
    <section className="program-section" id="program">
      <div className="wrap">
        <div className="section-heading">
          <h2>Программа по модулям</h2>
          <p>Выберите тему по учебнику, подготовьтесь в своём темпе и запишитесь на практику.</p>
        </div>
        <div className="program-switch" aria-label="Язык программы">
          {Object.entries(languages).map(([id, label]) => (
            <button type="button" className="button" onClick={() => setLanguage(id)} aria-pressed={id === language} key={id}>{label}</button>
          ))}
        </div>
        {school?.sample && <p className="muted sample-label">Пример программы и модулей</p>}
        {books.map(book => <div className="book-block" key={book.id}>
          <header className="book-heading">
            <span className="option-icon"><BookOpen size={22} /></span>
            <div>
              <h3>{book.title}</h3><p className="muted">{book.details}</p>
              {book.url && <a className="text-button" href={book.url} target="_blank" rel="noreferrer">Где найти учебник</a>}
            </div>
          </header>
          <div className="duo module-grid">
            {school.modules.filter(item => item.bookId === book.id).map(item => (
              <article className="module-card" key={item.id}>
                <p className="module-meta">{item.chapter} · {levels[item.level]}</p>
                <h4>{item.title}</h4>
                <p>{item.outcome}</p>
                <div className="module-actions">
                  <button className="button button-outline" onClick={() => onModule(item)}>{item.open ? 'Открыть пример' : 'О модуле'}</button>
                  <button className="button button-primary" onClick={() => onBook('individual', item.language, item.id)}>Календарь</button>
                </div>
              </article>
            ))}
          </div>
        </div>)}
        {!school && <p className="muted">Загружаем программу…</p>}
        {school && !books.length && <p className="muted">Модули этого языка появятся после обновления программы.</p>}
      </div>
    </section>
  )
}

export function TeacherIntro({ school, onBook }) {
  if (!school) return null
  return (
    <section className="wrap section" id="teacher">
      <div className="teacher-intro">
        <div className="teacher-mark">
          {school.teacher.photoUrl ? <img src={school.teacher.photoUrl} alt={school.teacher.name} loading="lazy" width="105" height="105" />
            : <GraduationCap size={37} strokeWidth={1.65} />}
        </div>
        <div>
          <p className="module-meta">{(school.teacher.languages || []).map(key => languages[key]).filter(Boolean).join(' · ') || 'Преподаватель'}</p>
          <h2>{school.teacher.name}</h2>
          <p>{school.teacher.bio}</p>
          <div className="module-actions">
            <button className="button button-primary" onClick={() => onBook('trial')}>Познакомиться на пробном</button>
            {school.supportUrl && <a className="button button-outline" href={school.supportUrl}><MessageCircle size={17} /> Написать</a>}
          </div>
        </div>
      </div>
    </section>
  )
}

export function ModuleDetails({ item, school, account, onPrepare, onBook, busy, error }) {
  const book = school.books.find(book => book.id === item.bookId)
  const progress = account?.progress?.find(p => p.moduleId === item.id)
  const requirements = (item.requires || []).map(id => school.modules.find(module => module.id === id)?.title).filter(Boolean)
  return (
    <div className="dialog-form module-detail">
      <p className="muted">{book?.title} · {item.chapter} · {levels[item.level]}</p>
      <div className="module-outcome"><strong>Что вы освоите</strong><p>{item.outcome}</p></div>
      <p className="muted">{requirements.length > 0 ? 'Перед началом пригодится: ' + requirements.join(', ') : 'Модуль можно начать с этой темы.'}</p>
      {(item.open || account) && <>
        <div className="module-prep"><strong><NotebookPen size={18} /> Самостоятельная подготовка</strong><p>{item.preparation || 'Откройте указанную главу учебника.'}</p>
          <small><Clock3 size={15} /> Около {item.minutes} минут</small></div>
        {item.materialUrl && <a className="button button-outline" href={item.materialUrl} target="_blank" rel="noreferrer">Открыть материалы</a>}
      </>}
      <ErrorMessage>{error}</ErrorMessage>
      {account && <button type="button" className="button button-outline" onClick={() => onPrepare(item, !progress?.prepared)} disabled={busy}>
        {progress?.prepared ? <><Check size={18} /> Подготовлено</> : 'Отметить подготовку'}
      </button>}
      <div className="module-actions">
        <button className="button button-primary" onClick={() => onBook('individual', item.language, item.id)}>Выбрать время</button>
        <button className="button button-outline" onClick={() => onBook('group', item.language, item.id)}>Найти мини-группу</button>
      </div>
    </div>
  )
}

export function LearningPanel({ account, school, onModule }) {
  const modules = school?.modules || []
  const progress = account.progress || []
  const recent = progress.find(item => item.nextModuleId)
  const next = modules.find(item => item.id === recent?.nextModuleId)
  const upcoming = account.bookings.find(item => !item.cancelled && item.moduleId && Date.parse(item.slot.startsAt) > Date.now())
  const current = modules.find(item => item.id === upcoming?.moduleId)
  return (
    <section className="cab-panel">
      <div className="panel-heading"><h2>Моё обучение</h2></div>
      {current && <div className="next-module">
        <div><p className="module-meta">К ближайшему занятию</p><h3>{current.title}</h3></div>
        <button className="button button-primary" onClick={() => onModule(current)}>Открыть подготовку</button>
      </div>}
      {next && <div className="next-module">
        <div><p className="module-meta">Рекомендация преподавателя</p><h3>{next.title}</h3></div>
        <button className="button button-primary" onClick={() => onModule(next)}>Открыть модуль</button>
      </div>}
      {progress.length === 0 && !current && !next && <div className="empty">
        <p>Выберите модуль и отметьте самостоятельную подготовку.</p>
        <a className="button button-outline" href="/#program">Посмотреть программу</a>
      </div>}
      <div className="lesson-list">
        {progress.map(item => {
          const module = modules.find(module => module.id === item.moduleId)
          if (!module) return null
          return <article className="progress-card" key={item.moduleId}>
            <div className="progress-heading"><div><p className="module-meta">{languages[module.language]} · {module.chapter}</p><h3>{module.title}</h3></div>
              <span className="status">{item.completed ? 'Пройден' : item.prepared ? 'Подготовлено' : 'В работе'}</span>
            </div>
            {item.feedback && <p className="feedback-text">{item.feedback}</p>}
            <button className="button button-outline" onClick={() => onModule(module)}>Открыть модуль</button>
          </article>
        })}
      </div>
    </section>
  )
}
