/**
 * @file
 * Benchmarks.
 */
namespace bench {
/**
 * @defgroup bench_group Benchmarks
 * @{
 */

/**
 * A flat array of numbers.
 *
 * @benchmark_run{ToCoreJson/FlatArray}
 * @benchmarks{lib::detail::to_core_json}
 */
static void ToCoreJsonFlatArray(int &state) {}

/**
 * Validation, parameterised on instance size.
 *
 * @benchmark_run{BlazeBackend/Validate}
 * @benchmarks{lib::BasicSchema::validate}
 * @benchmarks{lib::detail::to_core_json}
 */
static void Validate(int &state) {}

/**
 * Tagged, but never run.
 *
 * @benchmark_run{Never/Run}
 */
static void NeverRun(int &state) {}

/** @} */
} // namespace bench
