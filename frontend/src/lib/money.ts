const eur = new Intl.NumberFormat("en", { style: "currency", currency: "EUR" });

/** "€4.36". Prices are Cardmarket EUR trend figures; the API already rounds to cents. */
export function formatEUR(n: number): string {
  return eur.format(n);
}
