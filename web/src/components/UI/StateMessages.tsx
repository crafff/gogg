import styles from "./StateMessages.module.css";

interface StateMessageProps {
  message: string;
  type: "loading" | "error" | "empty";
}

export function StateMessage({ message, type }: StateMessageProps) {
  return (
    <div className={`${styles.message} ${styles[type]}`}>
      {type === "loading" && <span className={styles.spinner} />}
      {message}
    </div>
  );
}
