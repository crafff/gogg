import type { ReactNode } from "react";
import styles from "./MainLayout.module.css";

interface MainLayoutProps {
  children: ReactNode;
}

export function MainLayout({ children }: MainLayoutProps) {
  return (
    <div className={styles.container}>
      <main className={styles.main}>
        {children}
      </main>
      <footer className={styles.footer}>
        <p>&copy; 2024 GOGG. All rights reserved.</p>
      </footer>
    </div>
  );
}
