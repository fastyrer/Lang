import {
  CalendarDays, Check, BookOpen, Headphones, Menu, MessageCircle, MessagesSquare,
  UserRound, UsersRound, Video, X,
} from 'lucide-react'
import { useState } from 'react'
import { languages, lessonLabel, money } from './lib'
import { Program, TeacherIntro } from './Learning'

export function Logo() {
  return (
    <a className="logo" href="/" aria-label="Lingua, главная">
      <span className="logo-mark"><MessagesSquare size={23} strokeWidth={2.2} /></span>
      <span>lingua<span className="logo-dot">.</span></span>
    </a>
  )
}

const links = [['program', 'Программа'], ['formats', 'Форматы'], ['plans', 'Пакеты']]

export function Header({ account, cabinet, onAccount, onBook }) {
  const [menu, setMenu] = useState(false)
  return (
    <header className="site-header">
      <div className="wrap header-row">
        <Logo />
        <nav id="site-navigation" className={menu ? 'site-nav is-open' : 'site-nav'} aria-label="Основное меню">
          {links.map(([id, label]) => <a key={id} href={(cabinet ? '/' : '') + '#' + id} onClick={() => setMenu(false)}>{label}</a>)}
          <button className="button button-outline mobile-account" onClick={() => { setMenu(false); onAccount() }}>
            <UserRound size={17} /> {account ? 'Мой кабинет' : 'Войти'}
          </button>
        </nav>
        <div className="header-actions">
          <button className="button button-outline desktop-account" onClick={onAccount}>
            <UserRound size={16} /> {account ? 'Мой кабинет' : 'Войти'}
          </button>
          <button className="button button-primary header-book" onClick={() => onBook('trial')}>Записаться</button>
          <button className="icon-button menu-toggle" type="button" aria-label={menu ? 'Закрыть меню' : 'Открыть меню'}
            aria-expanded={menu} aria-controls="site-navigation" onClick={() => setMenu(!menu)}>
            {menu ? <X size={21} /> : <Menu size={21} />}
          </button>
        </div>
      </div>
    </header>
  )
}

function Hero({ onBook }) {
  return (
    <section className="wrap hero">
      <div className="hero-copy">
        <p className="eyebrow"><span /> Онлайн-школа языков</p>
        <h1>Говорите на<br /> <em>новом языке.</em></h1>
        <p className="hero-intro">Английский и китайский по учебникам. Выбирайте модули, готовьтесь в своём темпе и практикуйтесь с преподавателем.</p>
        <div className="hero-actions">
          <button className="button button-primary button-large" onClick={() => onBook('trial')}>
            <CalendarDays size={19} /> Попробовать бесплатно
          </button>
          <a className="button button-outline button-large" href="#program">Посмотреть модули</a>
        </div>
      </div>
      <div className="hero-media">
        <img src="/images/lesson.webp" alt="Девушка в наушниках занимается онлайн за ноутбуком" width="1320" height="990" fetchPriority="high" />
        <div className="greeting greeting-chinese">
          <span className="greeting-icon"><MessageCircle size={19} /></span>
          <span><strong lang="zh">你好!</strong><small lang="zh-Latn">Nǐ hǎo!</small></span>
        </div>
        <div className="greeting greeting-english">
          <span className="greeting-icon"><MessageCircle size={19} /></span>
          <span><strong lang="en">Hello!</strong><small>Привет!</small></span>
        </div>
      </div>
    </section>
  )
}

const courses = [
  { id: 'english', native: 'ENGLISH', symbol: 'Aa', lang: 'en', copy: 'Для путешествий, работы и разговоров в повседневной жизни.' },
  { id: 'chinese', native: '中文', symbol: '你好', lang: 'zh', copy: 'От первых слов и тонов до диалогов на современные темы.' },
]

const options = [
  { id: 'individual', icon: UserRound, title: 'Индивидуальные уроки', copy: 'Занимайтесь с преподавателем в своём темпе. Программу составим вместе с учётом ваших целей.', details: ['1 ученик', '55 минут'] },
  { id: 'group', icon: UsersRound, title: 'Занятия в мини-группах', copy: 'Практикуйте разговоры в парах и обсуждайте темы с учениками вашего уровня.', details: ['До 5 учеников', '55 минут'] },
]

const features = [
  { icon: Video, text: 'Живые онлайн-уроки' },
  { icon: MessagesSquare, text: 'Разговорная практика' },
  { icon: BookOpen, text: 'Модули по учебникам' },
]

export function PlanCards({ plans, onPlan, context, paymentMode }) {
  return (
    <div className="duo plan-grid">
      {plans.map(plan => {
        const Icon = plan.format === 'group' ? UsersRound : UserRound
        return (
          <article key={plan.id} className={'plan-card ' + (plan.format === 'group' ? 'plan-blue' : '')}>
            <div className="option-icon"><Icon size={24} /></div>
            <h3>{plan.name}</h3>
            <p className="plan-price">{money(plan.price)} <span>/ {plan.days} дней</span></p>
            <ul>
              <li><Check size={17} /> {lessonLabel(plan.lessons)} по 55 минут</li>
              <li><Check size={17} /> Английский или китайский</li>
            </ul>
            <button className={'button ' + (plan.format === 'group' ? 'button-primary' : 'button-outline')} onClick={() => onPlan(plan)}>
              {context === 'cabinet' && paymentMode === 'demo' ? 'Активировать' : 'Выбрать пакет'}
            </button>
          </article>
        )
      })}
    </div>
  )
}

export function Home({ catalog, catalogError, onBook, onPlan, onReload, onModule }) {
  return (
    <main id="main">
      <Hero onBook={onBook} />
      <div className="wrap">
        <ul className="features">
          {features.map(({ icon: Icon, text }) => <li key={text}><Icon size={20} /> {text}</li>)}
        </ul>
      </div>

      <Program school={catalog?.school} onModule={onModule} onBook={onBook} />

      <section className="wrap section" id="courses">
        <div className="section-heading">
          <h2>Выберите свой язык</h2>
          <p>Для тех, кто начинает с нуля или хочет говорить увереннее.</p>
        </div>
        <div className="duo">
          {courses.map(course => (
            <article key={course.id} className={'course-card course-' + course.id}>
              <span aria-hidden="true" className="course-symbol" lang={course.lang}>{course.symbol}</span>
              <div className="course-content">
                <span className="course-native" lang={course.lang}>{course.native}</span>
                <h3>{languages[course.id]}</h3>
                <p>{course.copy}</p>
                <button className="button button-white" onClick={() => onBook('trial', course.id)}>Попробовать урок</button>
              </div>
            </article>
          ))}
        </div>
      </section>

      <TeacherIntro school={catalog?.school} onBook={onBook} />

      <section className="format-section" id="formats">
        <div className="wrap">
          <div className="section-heading">
            <h2>Как вам удобно учиться?</h2>
            <p>Оба формата доступны для английского и китайского.</p>
          </div>
          <div className="duo">
            {options.map(({ id, icon: Icon, title, copy, details }) => (
              <article key={id} className="format-card">
                <div className="option-icon"><Icon size={24} /></div>
                <h3>{title}</h3>
                <p>{copy}</p>
                <ul className="details">{details.map(detail => <li key={detail}>{detail}</li>)}</ul>
                <button className="button button-outline" onClick={() => onBook(id)}>Смотреть расписание</button>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="wrap section plans-section" id="plans">
        <div className="section-heading">
          <h2>Пакеты занятий</h2>
          <p>Четыре урока на 30 дней. {catalog?.payments === 'demo' ? 'Демо-пакет активируется бесплатно.' : 'Разовая покупка.'}</p>
        </div>
        {catalog
          ? <PlanCards plans={catalog.plans} onPlan={onPlan} paymentMode={catalog.payments} />
          : <div className="empty">{catalogError || 'Загружаем абонементы…'} {catalogError && <button className="button button-outline" onClick={onReload}>Обновить</button>}</div>}
      </section>

      <section className="wrap closing-wrap">
        <div className="closing">
          <div>
            <h2>Начнём со знакомства?</h2>
            <p>За 25 минут обсудим вашу цель, определим уровень и подберём стартовый модуль.</p>
          </div>
          <button className="button button-white button-large" onClick={() => onBook('trial')}><Headphones size={19} /> Записаться на пробный</button>
        </div>
      </section>
      <section className="wrap section faq-section" id="faq">
        <div className="section-heading"><h2>Перед первым занятием</h2></div>
        <details><summary>Как выбрать модуль?</summary><p>Откройте программу нужного языка и посмотрите темы и требования. На пробном уроке преподаватель поможет найти точку входа.</p></details>
        <details><summary>Что будет, если планы изменятся?</summary><p>{catalog?.school?.cancelHours > 0
          ? 'Урок можно перенести или отменить в кабинете не позднее чем за ' + catalog.school.cancelHours + ' ч до начала.'
          : 'Урок можно перенести или отменить в кабинете до начала.'} При своевременной отмене урок возвращается на действующий пакет.</p></details>
        <details><summary>Как подключиться?</summary><p>Откройте подтверждённое занятие в личном кабинете и нажмите «Открыть встречу». Там же можно добавить урок в свой календарь.</p></details>
        <details><summary>Как работает пакет?</summary><p>Пакет действует 30 дней с активации и даёт четыре урока выбранного формата. За новым пакетом вы обращаетесь сами. {catalog?.school?.supportUrl && <a className="text-button" href={catalog.school.supportUrl}>Связаться по вопросам возврата</a>}</p></details>
      </section>
    </main>
  )
}

export function Footer({ cabinet, school }) {
  return (
    <footer className="footer">
      <div className="wrap footer-row">
        <Logo />
        <p>Онлайн-уроки английского и китайского</p>
        <div className="footer-links">
          <a href={(cabinet ? '/' : '') + '#faq'}>Вопросы</a>
          {school?.supportUrl && <a href={school.supportUrl}>Связаться</a>}
          {school?.privacyUrl && <a href={school.privacyUrl}>Конфиденциальность</a>}
          {school?.termsUrl && <a href={school.termsUrl}>Условия</a>}
        </div>
      </div>
    </footer>
  )
}
