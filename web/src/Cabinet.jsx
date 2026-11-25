import { BookOpen, CalendarDays, ChevronDown, CreditCard, LogOut, UserRound, Video } from 'lucide-react'
import { useState } from 'react'
import { PlanCards } from './Home'
import { ErrorMessage } from './Dialogs'
import { addToCalendar, dateLabel, formats, languages, lessonLabel, levels, slotLabel, timeLabel } from './lib'
import { LearningPanel } from './Learning'
import { Teaching } from './Teaching'

const tabs = [
  { id: 'lessons', label: 'Мои занятия', icon: CalendarDays },
  { id: 'learning', label: 'Моё обучение', icon: BookOpen },
  { id: 'subscription', label: 'Пакет занятий', icon: CreditCard },
  { id: 'profile', label: 'Профиль', icon: UserRound },
]

function Lesson({ booking, onCancel, demoSchedule, school, onMove, onModule }) {
  const { slot } = booking
  const now = Date.now()
  const upcoming = !booking.cancelled && Date.parse(slot.startsAt) > now
  const canChange = upcoming && Date.parse(booking.cancelBefore || slot.startsAt) > now
  const finished = Date.parse(slot.startsAt) + slot.duration * 60_000 <= now
  const module = school?.modules.find(item => item.id === booking.moduleId)
  const title = languages[slot.language] + ' · ' + formats[slot.format]
  return (
    <article className="lesson-card">
      <div className="option-icon"><CalendarDays size={22} /></div>
      <div className="lesson-copy">
        <h3>{title}</h3>
        {module && <button className="text-button lesson-module" onClick={() => onModule(module)}>{module.title}</button>}
        <p>{slotLabel(slot.startsAt)} · {timeLabel(slot.startsAt)} МСК · {slot.duration} мин</p>
        {booking.cancelled && <small>Занятие отменено преподавателем</small>}
        {!booking.cancelled && !finished && !slot.meetingUrl && <small>{demoSchedule ? 'Демо-занятие' : 'Ссылка пока недоступна'}</small>}
      </div>
      <div className="lesson-actions">
        {!booking.cancelled && !finished && slot.meetingUrl && <a className="button button-primary" href={slot.meetingUrl} target="_blank" rel="noreferrer"><Video size={17} /> Открыть встречу</a>}
        {upcoming && <button className="button button-outline" onClick={() => addToCalendar(booking, (module?.title || title) + ' · Lingua')}>В свой календарь</button>}
        {canChange && <button className="button button-outline" onClick={() => onMove(booking)}>Перенести</button>}
        {canChange && <button className="button button-outline" onClick={() => onCancel(booking)}>Отменить запись</button>}
        {upcoming && !canChange && school?.supportUrl && <a className="text-button" href={school.supportUrl}>Связаться со школой</a>}
      </div>
    </article>
  )
}

function MyLessons({ account, catalog, onBook, onCancel, onMove, onModule }) {
  const isFuture = item => !item.cancelled && Date.parse(item.slot.startsAt) + item.slot.duration * 60_000 > Date.now()
  const upcoming = account.bookings.filter(isFuture)
  const cancelled = account.bookings.filter(item => item.cancelled && Date.parse(item.slot.startsAt) > Date.now())
  const history = account.bookings.filter(item => !isFuture(item) && !cancelled.includes(item)).reverse()
  const hasTrial = account.bookings.some(item => !item.cancelled && item.slot.format === 'trial')
  const activePlan = account.subscription?.active ? account.subscription.planId : 'group'
  const firstFormat = hasTrial ? activePlan : 'trial'
  return (
    <section className="cab-panel">
      <div className="panel-heading">
        <h2>Мои занятия</h2>
        <button className="button button-primary" onClick={() => onBook(firstFormat, account.profile.language || 'english')}><CalendarDays size={17} /> Записаться</button>
      </div>
      {upcoming.length ? <div className="lesson-list">
        {upcoming.map(booking => <Lesson key={booking.id} booking={booking} demoSchedule={catalog?.demoSchedule} school={catalog?.school} onCancel={onCancel} onMove={onMove} onModule={onModule} />)}
      </div> : <div className="empty empty-lessons">
        <span className="option-icon"><CalendarDays size={25} /></span>
        <h3>Предстоящих занятий пока нет</h3>
        <p>Выберите язык и удобное время в расписании.</p>
      </div>}
      {cancelled.length > 0 && <div className="lesson-list cancelled-lessons">
        <p className="info" role="status">Преподаватель отменил занятие. Вы можете выбрать другое время.</p>
        {cancelled.map(booking => <Lesson key={booking.id} booking={booking} school={catalog?.school} onModule={onModule} />)}
      </div>}
      {history.length > 0 && <details className="lesson-history">
        <summary>Прошедшие занятия <ChevronDown size={18} /></summary>
        <div className="lesson-list">
          {history.map(booking => <Lesson key={booking.id} booking={booking} demoSchedule={catalog?.demoSchedule} school={catalog?.school} onCancel={onCancel} onMove={onMove} onModule={onModule} />)}
        </div>
      </details>}
    </section>
  )
}

function MySubscription({ account, catalog, onPlan, onCancel, onBook, catalogError, onReload, onRefreshOrder, busy }) {
  const sub = account.subscription
  const plan = catalog?.plans.find(plan => plan.id === sub?.planId)
  return (
    <section className="cab-panel">
      <div className="panel-heading"><h2>Пакет занятий</h2></div>
      {account.pendingOrder && <div className="pending-payment">
        <p>Заказ ожидает подтверждения оплаты.</p>
        <div className="module-actions">
          {account.pendingOrder.url && <a className="button button-primary" href={account.pendingOrder.url}>Продолжить оплату</a>}
          <button className="button button-outline" disabled={busy} onClick={() => onRefreshOrder(account.pendingOrder.id)}>Проверить оплату</button>
        </div>
      </div>}
      {sub?.active
        ? <div className="my-plan">
          <div className="my-plan-top">
            <span className="option-icon"><CreditCard size={23} /></span>
            <div><h3>{plan?.name || formats[sub.planId]}</h3><p>Действует до {dateLabel(sub.endsAt)}</p></div>
            <span className="status">Активен</span>
          </div>
          <div className="credit-line">
            <span>Осталось {lessonLabel(sub.remaining)}</span>
            {plan && <progress value={sub.remaining} max={plan.lessons}>{sub.remaining} из {plan.lessons}</progress>}
          </div>
          <div className="my-plan-actions">
            <button className="button button-primary" onClick={() => onBook(sub.planId, account.profile.language || 'english')}>Выбрать занятие</button>
            {catalog?.payments === 'demo' && <button className="button button-outline" onClick={onCancel}>Отменить демо-пакет</button>}
            {catalog?.payments !== 'demo' && catalog?.school?.supportUrl && <a className="button button-outline" href={catalog.school.supportUrl}>Связаться по оплате</a>}
          </div>
        </div>
        : <>
          <p className="muted cab-explanation">{sub?.cancelled ? 'Абонемент отменён. Вы можете активировать новый.' : 'Выберите формат занятий и подходящий пакет.'}</p>
          {catalog ? <PlanCards plans={catalog.plans} onPlan={onPlan} context="cabinet" paymentMode={catalog.payments} />
            : <div className="empty">{catalogError || 'Загружаем абонементы…'} {catalogError && <button className="button button-outline" onClick={onReload}>Обновить</button>}</div>}
        </>}
    </section>
  )
}

function Profile({ profile, onSave, busy }) {
  const [result, setResult] = useState({})
  async function submit(event) {
    event.preventDefault()
    setResult({})
    try {
      await onSave(Object.fromEntries(new FormData(event.currentTarget)))
      setResult({ saved: true })
    } catch (error) {
      setResult({ error: error.message })
    }
  }
  return (
    <section className="cab-panel">
      <div className="panel-heading"><h2>Профиль</h2></div>
      <form className="profile-form" onSubmit={submit} onChange={() => setResult({})}>
        <label>Имя<input name="name" defaultValue={profile.name} autoComplete="name" minLength={2} maxLength={60} required /></label>
        <label>Почта<input type="email" value={profile.email} readOnly /></label>
        <label>Язык<select name="language" defaultValue={profile.language}>
          <option value="">Выберите язык</option>
          {Object.entries(languages).map(([id, label]) => <option key={id} value={id}>{label}</option>)}
        </select></label>
        <label>Уровень<select name="level" defaultValue={profile.level}>
          <option value="">Выберите уровень</option>
          {Object.entries(levels).map(([id, label]) => <option key={id} value={id}>{label}</option>)}
        </select></label>
        <ErrorMessage>{result.error}</ErrorMessage>
        {result.saved && <p role="status" className="success">Профиль сохранён.</p>}
        <button className="button button-primary" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить'}</button>
      </form>
    </section>
  )
}

export function Cabinet({ account, catalog, catalogError, tab, setTab, onBook, onPlan, onCancelBooking, onMove, onModule, onCancelPlan, onSave, onLogout, onReload, onRefreshOrder, busy, notice }) {
  const links = account.profile.teacher ? [{ id: 'teaching', label: 'Преподаватель', icon: CalendarDays }, ...tabs] : tabs
  return (
    <main id="main" className="wrap cabinet">
      <div className="cab-welcome">
        <div className="avatar" aria-hidden="true">{Array.from(account.profile.name)[0]?.toLocaleUpperCase('ru')}</div>
        <div><p>Личный кабинет</p><h1>{account.profile.name}</h1></div>
      </div>
      {notice && <p className="success cab-notice" role="status">{notice}</p>}
      <div className="cab-layout">
        <nav className="cab-nav" aria-label="Личный кабинет">
          {links.map(({ id, label, icon: Icon }) => <button className={tab === id ? 'current' : ''} aria-current={tab === id ? 'page' : undefined} onClick={() => setTab(id)} key={id}><Icon size={19} /> {label}</button>)}
          <button className="logout" aria-label="Выйти" onClick={onLogout} disabled={busy}><LogOut size={19} /><span>Выйти</span></button>
        </nav>
        {tab === 'lessons' && <MyLessons account={account} catalog={catalog} onBook={onBook} onCancel={onCancelBooking} onMove={onMove} onModule={onModule} />}
        {tab === 'learning' && <LearningPanel account={account} school={catalog?.school} onModule={onModule} />}
        {tab === 'teaching' && account.profile.teacher && <Teaching catalog={catalog} onUpdate={onReload} />}
        {tab === 'subscription' && <MySubscription account={account} catalog={catalog} catalogError={catalogError} onPlan={onPlan} onCancel={onCancelPlan} onBook={onBook} onReload={onReload} onRefreshOrder={onRefreshOrder} busy={busy} />}
        {tab === 'profile' && <Profile profile={account.profile} onSave={onSave} busy={busy} />}
      </div>
    </main>
  )
}
