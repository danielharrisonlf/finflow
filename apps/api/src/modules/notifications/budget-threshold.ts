/**
 * Limiar de alerta cruzado por um novo gasto: 90 (crítico), 70 (atenção) ou null.
 * Alerta só dispara no cruzamento — gastos seguintes acima do limiar não repetem o aviso.
 */
export function crossedThreshold(beforePercent: number, afterPercent: number): 90 | 70 | null {
  if (beforePercent < 90 && afterPercent >= 90) return 90;
  if (beforePercent < 70 && afterPercent >= 70) return 70;
  return null;
}
