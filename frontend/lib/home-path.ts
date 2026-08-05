/** Home route after login — sites first, including master mode. */
export async function resolveHomePath(): Promise<"/sites"> {
  return "/sites";
}
