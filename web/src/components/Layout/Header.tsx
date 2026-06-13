import styles from "./Header.module.css";

interface HeaderProps {
  title?: string;
  subtitle?: string;
}

export function Header({ 
  title = "峡谷英雄强度榜", 
  subtitle = `${new Date().toISOString().slice(0, 10)}  数据来源：对局统计` 
}: HeaderProps) {
  return (
    <header className={styles.header}>
      <h1 className={styles.title}>{title}</h1>
      {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
    </header>
  );
}
