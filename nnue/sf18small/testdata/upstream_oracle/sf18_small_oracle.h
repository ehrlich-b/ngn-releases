#ifndef SF18_SMALL_ORACLE_H_INCLUDED
#define SF18_SMALL_ORACLE_H_INCLUDED

#include <algorithm>
#include <array>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <limits>
#include <memory>
#include <sstream>
#include <string>

#include "network.h"
#include "nnue_accumulator.h"
#include "nnue_architecture.h"
#include "nnue_feature_transformer.h"
#include "../position.h"

namespace Stockfish::Eval::NNUE {
namespace SF18SmallOracle {

struct Bounds {
    std::int64_t min = std::numeric_limits<std::int64_t>::max();
    std::int64_t max = std::numeric_limits<std::int64_t>::min();
    void add(std::int64_t value) {
        min = std::min(min, value);
        max = std::max(max, value);
    }
};

inline std::int16_t wrap16(std::int64_t value) {
    const auto bits = static_cast<std::uint16_t>(value);
    std::int16_t result;
    std::memcpy(&result, &bits, sizeof(result));
    return result;
}

inline std::int32_t wrap32(std::int64_t value) {
    const auto bits = static_cast<std::uint32_t>(value);
    std::int32_t result;
    std::memcpy(&result, &bits, sizeof(result));
    return result;
}

template<typename T, std::size_t N>
void json_array(std::ostream& out, const std::array<T, N>& values) {
    out << '[';
    for (std::size_t i = 0; i < N; ++i)
    {
        if (i)
            out << ',';
        out << +values[i];
    }
    out << ']';
}

template<typename Layer, std::size_t N>
std::pair<std::int32_t, Bounds>
affine_shadow_row(const Layer& layer, std::size_t row, const std::array<std::uint8_t, N>& input) {
    std::int32_t current = layer.biases[row];
    Bounds       bounds;
    bounds.add(current);
    for (std::size_t i = 0; i < Layer::InputDimensions; ++i)
    {
        const auto logical = row * Layer::PaddedInputDimensions + i;
        const auto weight  = layer.weights[Layer::get_weight_index(logical)];
        const auto wide    = std::int64_t(current) + std::int64_t(input[i]) * weight;
        bounds.add(wide);
        current = wrap32(wide);
    }
    return {current, bounds};
}

inline void merge(Bounds& into, const Bounds& from) {
    into.add(from.min);
    into.add(from.max);
}

struct StackTrace {
    std::array<std::int32_t, 16> fc0{};
    std::array<std::uint8_t, 15> squared{};
    std::array<std::uint8_t, 15> clipped0{};
    std::array<std::int32_t, 32> fc1{};
    std::array<std::uint8_t, 32> clipped1{};
    std::int32_t                 fc2{};
    std::int32_t                 forward{};
    std::int32_t                 positionalRaw{};
    Bounds                       fc0Bounds;
    Bounds                       fc1Bounds;
    Bounds                       fc2Bounds;
    std::int64_t                 forwardWide{};
    std::int64_t                 positionalWide{};
};

inline StackTrace trace_stack(const SmallNetworkArchitecture& architecture,
                              const std::array<std::uint8_t, 128>& input) {
    StackTrace result;
    alignas(CacheLineSize) std::int32_t fc0Actual[32]{};
    alignas(CacheLineSize) std::uint8_t squaredActual[32]{};
    alignas(CacheLineSize) std::uint8_t clipped0Actual[32]{};
    alignas(CacheLineSize) std::uint8_t fc1Input[32]{};
    alignas(CacheLineSize) std::int32_t fc1Actual[32]{};
    alignas(CacheLineSize) std::uint8_t clipped1Actual[32]{};
    alignas(CacheLineSize) std::int32_t fc2Actual[32]{};

    architecture.fc_0.propagate(input.data(), fc0Actual);
    architecture.ac_sqr_0.propagate(fc0Actual, squaredActual);
    architecture.ac_0.propagate(fc0Actual, clipped0Actual);
    std::copy_n(squaredActual, 15, fc1Input);
    std::copy_n(clipped0Actual, 15, fc1Input + 15);
    architecture.fc_1.propagate(fc1Input, fc1Actual);
    architecture.ac_1.propagate(fc1Actual, clipped1Actual);
    architecture.fc_2.propagate(clipped1Actual, fc2Actual);

    std::copy_n(fc0Actual, 16, result.fc0.begin());
    std::copy_n(squaredActual, 15, result.squared.begin());
    std::copy_n(clipped0Actual, 15, result.clipped0.begin());
    std::copy_n(fc1Actual, 32, result.fc1.begin());
    std::copy_n(clipped1Actual, 32, result.clipped1.begin());
    result.fc2 = fc2Actual[0];

    for (std::size_t row = 0; row < 16; ++row)
    {
        const auto [shadow, bounds] = affine_shadow_row(architecture.fc_0, row, input);
        if (shadow != result.fc0[row])
            std::abort();
        merge(result.fc0Bounds, bounds);
    }
    std::array<std::uint8_t, 32> fc1InputArray{};
    std::copy_n(fc1Input, 32, fc1InputArray.begin());
    for (std::size_t row = 0; row < 32; ++row)
    {
        const auto [shadow, bounds] = affine_shadow_row(architecture.fc_1, row, fc1InputArray);
        if (shadow != result.fc1[row])
            std::abort();
        merge(result.fc1Bounds, bounds);
    }
    std::array<std::uint8_t, 32> clipped1Array{};
    std::copy_n(clipped1Actual, 32, clipped1Array.begin());
    const auto [fc2Shadow, fc2Bounds] = affine_shadow_row(architecture.fc_2, 0, clipped1Array);
    if (fc2Shadow != result.fc2)
        std::abort();
    result.fc2Bounds = fc2Bounds;

    result.forwardWide = std::int64_t(result.fc0[15]) * 600 * OutputScale;
    result.forward = result.fc0[15] * (600 * OutputScale) / (127 * (1 << WeightScaleBits));
    result.positionalWide = std::int64_t(result.fc2) + result.forward;
    result.positionalRaw = wrap32(result.positionalWide);
    if (architecture.propagate(input.data()) != result.positionalRaw)
        std::abort();
    return result;
}

inline void json_bounds(std::ostream& out, const char* prefix, const Bounds& bounds) {
    out << '\"' << prefix << "_wide_min\":" << bounds.min << ','
        << '\"' << prefix << "_wide_max\":" << bounds.max;
}

inline std::string sf18_small_oracle_json(const Position& position,
                                          const NetworkSmall& network,
                                          AccumulatorCaches::Cache<128>& cache) {
    auto stack = std::make_unique<AccumulatorStack>();
    stack->reset();
    cache.clear(network);

    std::array<std::uint8_t, 128> transformed{};
    network.featureTransformer.transform(position, *stack, cache, transformed.data(), 0);
    const auto& actual = stack->latest<PSQFeatureSet>().acc<128>();

    std::array<PSQFeatureSet::IndexList, 2> active;
    std::array<std::array<std::int16_t, 128>, 2> canonicalAccumulator{};
    std::array<Bounds, 2> accumulatorBounds;
    std::array<Bounds, 2> psqtBounds;

    for (Color perspective : {WHITE, BLACK})
    {
        PSQFeatureSet::append_active_indices(perspective, position, active[perspective]);
        canonicalAccumulator[perspective] = actual.accumulation[perspective];
        permute<16>(canonicalAccumulator[perspective],
                    SmallFeatureTransformer::InversePackusEpi16Order);

        auto logicalBiases = network.featureTransformer.biases;
        permute<16>(logicalBiases, SmallFeatureTransformer::InversePackusEpi16Order);
        auto shadow = logicalBiases;
        for (auto value : shadow)
            accumulatorBounds[perspective].add(value);
        for (const auto index : active[perspective])
        {
            std::array<std::int16_t, 128> logicalWeights{};
            std::copy_n(network.featureTransformer.weights.data() + index * 128,
                        128, logicalWeights.begin());
            permute<16>(logicalWeights, SmallFeatureTransformer::InversePackusEpi16Order);
            for (std::size_t lane = 0; lane < 128; ++lane)
            {
                const auto wide = std::int64_t(shadow[lane]) + logicalWeights[lane];
                accumulatorBounds[perspective].add(wide);
                shadow[lane] = wrap16(wide);
            }
        }
        if (shadow != canonicalAccumulator[perspective])
            std::abort();

        std::array<std::int32_t, 8> psqtShadow{};
        psqtBounds[perspective].add(0);
        for (const auto index : active[perspective])
            for (std::size_t bucket = 0; bucket < 8; ++bucket)
            {
                const auto wide = std::int64_t(psqtShadow[bucket])
                                + network.featureTransformer.psqtWeights[index * 8 + bucket];
                psqtBounds[perspective].add(wide);
                psqtShadow[bucket] = wrap32(wide);
            }
        if (psqtShadow != actual.psqtAccumulation[perspective])
            std::abort();
    }

    std::array<std::int32_t, 8> psqtRaw{};
    std::array<std::int64_t, 8> psqtDifferenceWide{};
    std::array<StackTrace, 8> stacks{};
    std::array<std::int32_t, 8> psqtOutput{};
    std::array<std::int32_t, 8> positionalOutput{};
    const Color stm = position.side_to_move();
    for (std::size_t bucket = 0; bucket < 8; ++bucket)
    {
        psqtDifferenceWide[bucket] = std::int64_t(actual.psqtAccumulation[stm][bucket])
                                   - actual.psqtAccumulation[~stm][bucket];
        psqtRaw[bucket] = wrap32(psqtDifferenceWide[bucket]) / 2;
        std::array<std::uint8_t, 128> bucketTransformed{};
        const auto actualPsqt = network.featureTransformer.transform(
          position, *stack, cache, bucketTransformed.data(), bucket);
        if (actualPsqt != psqtRaw[bucket] || bucketTransformed != transformed)
            std::abort();
        stacks[bucket] = trace_stack(network.network[bucket], transformed);
        psqtOutput[bucket] = psqtRaw[bucket] / OutputScale;
        positionalOutput[bucket] = stacks[bucket].positionalRaw / OutputScale;
    }

    cache.clear(network);
    auto productionStack = std::make_unique<AccumulatorStack>();
    productionStack->reset();
    const auto productionTrace = network.trace_evaluate(position, *productionStack, cache);
    if (productionTrace.correctBucket != (position.count<ALL_PIECES>() - 1) / 4)
        std::abort();
    for (std::size_t bucket = 0; bucket < 8; ++bucket)
        if (std::int32_t(productionTrace.psqt[bucket]) != psqtOutput[bucket]
            || std::int32_t(productionTrace.positional[bucket]) != positionalOutput[bucket])
            std::abort();

    std::ostringstream out;
    out << "{\"source_commit\":\"cb3d4ee9b47d0c5aae855b12379378ea1439675c\""
        << ",\"network_sha256\":\"37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d\""
        << ",\"fen\":\"" << position.fen() << "\",\"side_to_move\":" << int(stm)
        << ",\"pieces\":[";
    bool first = true;
    for (Square square = SQ_A1; square < SQUARE_NB; ++square)
    {
        const Piece piece = position.piece_on(square);
        if (piece == NO_PIECE)
            continue;
        if (!first)
            out << ',';
        first = false;
        out << "{\"Piece\":" << int(type_of(piece)) - 1 << ",\"Color\":"
            << int(color_of(piece)) << ",\"Square\":" << int(square) << '}';
    }
    out << "],\"correct_bucket\":" << (position.count<ALL_PIECES>() - 1) / 4;
    out << ",\"active\":[";
    for (Color perspective : {WHITE, BLACK})
    {
        if (perspective != WHITE)
            out << ',';
        out << '[';
        bool firstIndex = true;
        for (const auto index : active[perspective])
        {
            if (!firstIndex)
                out << ',';
            firstIndex = false;
            out << index;
        }
        out << ']';
    }
    out << "],\"accumulator\":[";
    json_array(out, canonicalAccumulator[WHITE]); out << ',';
    json_array(out, canonicalAccumulator[BLACK]); out << ']';
    out << ",\"psqt_accumulator\":[";
    json_array(out, actual.psqtAccumulation[WHITE]); out << ',';
    json_array(out, actual.psqtAccumulation[BLACK]); out << ']';
    out << ",\"transformed\":"; json_array(out, transformed);
    out << ",\"psqt_raw\":"; json_array(out, psqtRaw);
    out << ",\"psqt_difference_wide\":"; json_array(out, psqtDifferenceWide);
    out << ",\"accumulator_wide_min\":[" << accumulatorBounds[0].min << ',' << accumulatorBounds[1].min << ']';
    out << ",\"accumulator_wide_max\":[" << accumulatorBounds[0].max << ',' << accumulatorBounds[1].max << ']';
    out << ",\"psqt_wide_min\":[" << psqtBounds[0].min << ',' << psqtBounds[1].min << ']';
    out << ",\"psqt_wide_max\":[" << psqtBounds[0].max << ',' << psqtBounds[1].max << ']';
    out << ",\"stacks\":[";
    for (std::size_t bucket = 0; bucket < 8; ++bucket)
    {
        if (bucket)
            out << ',';
        const auto& s = stacks[bucket];
        out << "{\"fc0\":"; json_array(out, s.fc0);
        out << ",\"squared\":"; json_array(out, s.squared);
        out << ",\"clipped0\":"; json_array(out, s.clipped0);
        out << ",\"fc1\":"; json_array(out, s.fc1);
        out << ",\"clipped1\":"; json_array(out, s.clipped1);
        out << ",\"fc2\":" << s.fc2 << ",\"forward\":" << s.forward
            << ",\"positional_raw\":" << s.positionalRaw << ',';
        json_bounds(out, "fc0", s.fc0Bounds); out << ',';
        json_bounds(out, "fc1", s.fc1Bounds); out << ',';
        json_bounds(out, "fc2", s.fc2Bounds);
        out << ",\"forward_wide\":" << s.forwardWide
            << ",\"positional_wide\":" << s.positionalWide << '}';
    }
    out << "],\"components\":[";
    for (std::size_t bucket = 0; bucket < 8; ++bucket)
    {
        if (bucket)
            out << ',';
        out << "{\"PSQT\":" << psqtOutput[bucket]
            << ",\"Positional\":" << positionalOutput[bucket] << '}';
    }
    out << "]}";
    return out.str();
}

}  // namespace SF18SmallOracle

inline std::string sf18_small_oracle_json(const Position& position,
                                          const NetworkSmall& network,
                                          AccumulatorCaches::Cache<128>& cache) {
    return SF18SmallOracle::sf18_small_oracle_json(position, network, cache);
}

}  // namespace Stockfish::Eval::NNUE

#endif
