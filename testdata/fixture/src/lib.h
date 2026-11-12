/**
 * @file
 * The library under test.
 */
#include <concepts>
namespace lib {
/** Anything JSON-shaped. */
template <typename T> concept JsonView = requires(T t) { t.kind(); };

namespace detail {
/**
 * Converts a view to core JSON.
 * @param value The view.
 * @return The JSON.
 */
template <JsonView V> int to_core_json(const V &value) { return 0; }
} // namespace detail

/** A schema, parameterised on a backend. */
template <typename Backend> class BasicSchema {
  public:
    /** Validates an instance. */
    template <JsonView V> int validate(const V &instance) const { return 0; }
};
} // namespace lib
