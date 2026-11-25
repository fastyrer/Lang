import { CalendarDays, Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api, formats, languages, slotLabel, timeLabel } from './lib'
import { dateKey } from './Calendar'
import { Dialog, ErrorMessage } from './Dialogs'

function SlotForm({ selected, modules, onSave, busy, error }) {
  const [language, setLanguage] = useState(selected?.language || 'english')
  const [format, setFormat] = useState(selected?.format || 'individual')
  const start = selected ? dateKey(selected.startsAt) + 'T' + timeLabel(selected.startsAt) : ''
  return <form className="dialog-form" onSubmit={event => {
    event.preventDefault()
    const form = Object.fromEntries(new FormData(event.currentTarget))
    onSave({ ...form, startsAt: form.startsAt + ':00+03:00', duration: Number(form.duration), capacity: format === 'group' ? Number(form.capacity) : 1 })
  }}>
    <div className="booking-selects">
      <label>Язык<select name="language" value={language} onChange={event => setLanguage(event.target.value)}>{Object.entries(languages).map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></label>
      <label>Формат<select name="format" value={format} onChange={event => setFormat(event.target.value)}>{Object.entries(formats).map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></label>
    </div>
    <label>Модуль<select name="moduleId" key={language} defaultValue={selected?.language === language ? selected.moduleId : ''} required={format === 'group'}>
      <option value="">Выберите модуль</option>
      {modules.filter(item => item.language === language).map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
    </select></label>
    <label>Начало по Москве<input type="datetime-local" name="startsAt" defaultValue={start} required /></label>
    <div className="booking-selects">
      <label>Длительность, мин<input type="number" name="duration" min="10" max="180" defaultValue={selected?.duration || 55} required /></label>
      {format === 'group' && <label>Мест в группе<input type="number" name="capacity" min="2" max="5" defaultValue={selected?.capacity > 1 ? selected.capacity : 5} required /></label>}
    </div>
    <label>Ссылка на видеовстречу<input type="url" name="meetingUrl" defaultValue={selected?.meetingUrl || ''} placeholder="https://…" required /></label>
    <ErrorMessage>{error}</ErrorMessage>
    <button className="button button-primary" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить занятие'}</button>
  </form>
}

function FeedbackForm({ entry, modules, onSave, error, busy }) {
  const { booking, student, progress } = entry
  const [moduleId, setModuleId] = useState(booking.moduleId || '')
  const options = modules.filter(item => item.language === booking.slot.language)
  return <form className="dialog-form" onSubmit={event => {
    event.preventDefault()
    const form = Object.fromEntries(new FormData(event.currentTarget))
    onSave({ ...form, moduleId, completed: form.completed === 'on' })
  }}>
    <p className="muted">{student} · {slotLabel(booking.slot.startsAt)} · {timeLabel(booking.slot.startsAt)} МСК</p>
    <label>Модуль<select value={moduleId} onChange={event => setModuleId(event.target.value)} required disabled={Boolean(booking.moduleId)}>
      <option value="">Выберите модуль</option>
      {options.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
    </select></label>
    <label>Комментарий<textarea name="feedback" rows={4} maxLength={900} defaultValue={progress.feedback || ''} /></label>
    <label className="check-label"><input type="checkbox" name="completed" defaultChecked={progress.completed} className="small-check" /> Отметить модуль как пройденный</label>
    <label>Рекомендовать следующий модуль<select name="nextModuleId" key={moduleId} defaultValue={progress.nextModuleId || ''}>
      <option value="">Пока без рекомендации</option>
      {options.filter(item => item.id !== moduleId).map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
    </select></label>
    <ErrorMessage>{error}</ErrorMessage>
    <button className="button button-primary" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить отзыв'}</button>
  </form>
}

export function Teaching({ catalog, onUpdate }) {
  const [data, setData] = useState(null)
  const [modal, setModal] = useState(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const modules = catalog?.school?.modules || []

  async function load() {
    try { setData(await api('/teacher')) }
    catch (error) { setError(error.message) }
  }

  useEffect(() => { load() }, [])

  function show(next) { setError(''); setModal(next) }

  async function action(path, method, body) {
    setBusy(true)
    setError('')
    try {
      await api(path, { method, body })
      setModal(null)
      await load()
      onUpdate()
    } catch (error) {
      setError(error.message)
    } finally {
      setBusy(false)
    }
  }

  const currentSlot = modal?.slot
  const slotPath = currentSlot ? '/teacher/slots/' + currentSlot.id : '/teacher/slots'
  return <section className="cab-panel">
    <div className="panel-heading">
      <h2>Расписание преподавателя</h2>
      <button className="button button-primary" onClick={() => show({ kind: 'slot' })}><Plus size={18} /> Добавить занятие</button>
    </div>
    {!modal && <ErrorMessage>{error}</ErrorMessage>}
    {!data && !error && <p className="muted">Загружаем расписание…</p>}
    {data?.slots.length === 0 && <p className="info">Добавьте первое реальное занятие. Оно заменит демо-расписание.</p>}
    <div className="lesson-list">
      {data?.slots.map(slot => {
        const topic = modules.find(item => item.id === slot.moduleId)
        const count = data.bookings.filter(item => item.booking.slot.id === slot.id).length
        return <article className="teacher-slot" key={slot.id}>
          <div>
            <h3>{languages[slot.language]} · {formats[slot.format]}</h3>
            <p className="muted">{slotLabel(slot.startsAt)} · {timeLabel(slot.startsAt)} МСК · {count}/{slot.capacity} мест</p>
            {topic && <p className="muted">{topic.title}</p>}
          </div>
          {slot.managed && <div className="module-actions">
            <button className="button button-outline" onClick={() => show({ kind: 'slot', slot })}>Изменить</button>
            <button className="text-button" onClick={() => show({ kind: 'cancel', slot })}>Отменить</button>
          </div>}
        </article>
      })}
    </div>
    <h3 className="teaching-section-title"><CalendarDays size={19} /> Записи учеников</h3>
    {data?.bookings.length === 0 && <p className="muted">Подтверждённых записей пока нет.</p>}
    <div className="lesson-list">
      {data?.bookings.map(entry => {
        const { booking, student } = entry
        const topic = modules.find(item => item.id === booking.moduleId)
        return <article className="teacher-slot" key={booking.id}>
          <div><h3>{student}</h3><p className="muted">{topic?.title || languages[booking.slot.language]} · {slotLabel(booking.slot.startsAt)} · {timeLabel(booking.slot.startsAt)} МСК</p></div>
          {Date.parse(booking.slot.startsAt) <= Date.now() && <button className="button button-outline" onClick={() => show({ kind: 'feedback', entry })}>Обратная связь</button>}
        </article>
      })}
    </div>
    {modal && <Dialog title={{ slot: currentSlot ? 'Изменить занятие' : 'Новое занятие', cancel: 'Отменить занятие?', feedback: 'Обратная связь' }[modal.kind]} onClose={() => show(null)} busy={busy}>
      {modal.kind === 'slot' && <SlotForm selected={currentSlot} modules={modules} busy={busy} error={error} onSave={body => action(slotPath, currentSlot ? 'PUT' : 'POST', body)} />}
      {modal.kind === 'feedback' && <FeedbackForm entry={modal.entry} modules={modules} busy={busy} error={error} onSave={body => action('/teacher/feedback/' + modal.entry.booking.id, 'PATCH', body)} />}
      {modal.kind === 'cancel' && <div className="dialog-form">
        <p>Записи учеников будут отменены. Урок вернётся на действующий пакет, если был с него списан.</p>
        <ErrorMessage>{error}</ErrorMessage>
        <button className="button button-danger" disabled={busy} onClick={() => action(slotPath, 'DELETE')}>Подтвердить отмену</button>
      </div>}
    </Dialog>}
  </section>
}
