#ifndef SF18_BIG_REFERENCE_ORACLE_H_INCLUDED
#define SF18_BIG_REFERENCE_ORACLE_H_INCLUDED

#include <algorithm>
#include <array>
#include <cstdint>
#include <cstdlib>
#include <deque>
#include <memory>
#include <sstream>
#include <string>
#include <vector>

#include "network.h"
#include "nnue_accumulator.h"
#include "nnue_architecture.h"
#include "nnue_feature_transformer.h"
#include "../position.h"
#include "../tt.h"
#include "../uci.h"

namespace Stockfish::Eval::NNUE::SF18BigReferenceOracle {

template<typename Range>
void json_array(std::ostream& out, const Range& values) {
    out << '[';
    bool first = true;
    for (const auto value : values)
    {
        if (!first)
            out << ',';
        first = false;
        out << +value;
    }
    out << ']';
}

inline std::string json_string(const std::string& value) {
    std::ostringstream out;
    out << '"';
    for (const unsigned char c : value)
    {
        switch (c)
        {
        case '"': out << "\\\""; break;
        case '\\': out << "\\\\"; break;
        case '\b': out << "\\b"; break;
        case '\f': out << "\\f"; break;
        case '\n': out << "\\n"; break;
        case '\r': out << "\\r"; break;
        case '\t': out << "\\t"; break;
        default:
            if (c < 0x20)
            {
                constexpr char hex[] = "0123456789abcdef";
                out << "\\u00" << hex[c >> 4] << hex[c & 15];
            }
            else
                out << c;
        }
    }
    out << '"';
    return out.str();
}

using ThreatLists = std::array<ThreatFeatureSet::IndexList, COLOR_NB>;

inline ThreatLists active_threats(const Position& position) {
    ThreatLists result;
    for (Color perspective : {WHITE, BLACK})
        ThreatFeatureSet::append_active_indices(perspective, position, result[perspective]);
    return result;
}

inline void json_index_list(std::ostream& out, const ThreatFeatureSet::IndexList& values) {
    out << '[';
    bool first = true;
    for (const auto value : values)
    {
        if (!first)
            out << ',';
        first = false;
        out << value;
    }
    out << ']';
}

inline void json_index_pair(std::ostream& out, const ThreatLists& values) {
    out << '[';
    json_index_list(out, values[WHITE]);
    out << ',';
    json_index_list(out, values[BLACK]);
    out << ']';
}

inline std::string state_json(const Position&                           position,
                              const NetworkBig&                         network,
                              AccumulatorStack&                         stack,
                              AccumulatorCaches::Cache<1024>&           cache) {
    std::array<std::uint8_t, 1024> transformed{};
    const int bucket = (position.count<ALL_PIECES>() - 1) / 4;
    const auto psqtRaw = network.featureTransformer.transform(
      position, stack, cache, transformed.data(), bucket);

    const auto& baseActual = stack.latest<PSQFeatureSet>().acc<1024>();
    const auto& threatActual = stack.latest<ThreatFeatureSet>().acc<1024>();
    std::array<std::array<std::int16_t, 1024>, COLOR_NB> baseAccumulator =
      baseActual.accumulation;
    std::array<std::array<std::int16_t, 1024>, COLOR_NB> threatAccumulator =
      threatActual.accumulation;
    for (Color perspective : {WHITE, BLACK})
    {
        permute<16>(baseAccumulator[perspective], BigFeatureTransformer::InversePackusEpi16Order);
        permute<16>(threatAccumulator[perspective], BigFeatureTransformer::InversePackusEpi16Order);
    }

    const auto active = active_threats(position);
    const auto positionalRaw = network.network[bucket].propagate(transformed.data());
    std::ostringstream out;
    out << "{\"source_commit\":\"cb3d4ee9b47d0c5aae855b12379378ea1439675c\""
        << ",\"network_sha256\":\"c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7\""
        << ",\"fen\":" << json_string(position.fen())
        << ",\"side_to_move\":" << int(position.side_to_move())
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
        out << "{\"Piece\":" << int(type_of(piece)) - 1
            << ",\"Color\":" << int(color_of(piece))
            << ",\"Square\":" << int(square) << '}';
    }
    out << "],\"bucket\":" << bucket << ",\"active_threats\":";
    json_index_pair(out, active);
    out << ",\"base_accumulator\":[";
    json_array(out, baseAccumulator[WHITE]);
    out << ',';
    json_array(out, baseAccumulator[BLACK]);
    out << "],\"threat_accumulator\":[";
    json_array(out, threatAccumulator[WHITE]);
    out << ',';
    json_array(out, threatAccumulator[BLACK]);
    out << "],\"base_psqt\":[";
    json_array(out, baseActual.psqtAccumulation[WHITE]);
    out << ',';
    json_array(out, baseActual.psqtAccumulation[BLACK]);
    out << "],\"threat_psqt\":[";
    json_array(out, threatActual.psqtAccumulation[WHITE]);
    out << ',';
    json_array(out, threatActual.psqtAccumulation[BLACK]);
    out << "],\"transformed\":";
    json_array(out, transformed);
    out << ",\"psqt_raw\":" << psqtRaw
        << ",\"positional_raw\":" << positionalRaw
        << ",\"components\":{\"PSQT\":" << psqtRaw / OutputScale
        << ",\"Positional\":" << positionalRaw / OutputScale << "}}";
    return out.str();
}

inline void sorted_copy(const ThreatFeatureSet::IndexList& source,
                        std::vector<IndexType>&              target) {
    target.assign(source.begin(), source.end());
    std::sort(target.begin(), target.end());
}

inline void json_transition(std::ostream&          out,
                            const ThreatLists&     before,
                            const ThreatLists&     after,
                            const DirtyThreats*    dirty) {
    out << "{\"removed\":[";
    std::array<std::vector<IndexType>, COLOR_NB> removed;
    std::array<std::vector<IndexType>, COLOR_NB> added;
    for (Color perspective : {WHITE, BLACK})
    {
        std::vector<IndexType> beforeSorted;
        std::vector<IndexType> afterSorted;
        sorted_copy(before[perspective], beforeSorted);
        sorted_copy(after[perspective], afterSorted);
        std::set_difference(beforeSorted.begin(), beforeSorted.end(),
                            afterSorted.begin(), afterSorted.end(),
                            std::back_inserter(removed[perspective]));
        std::set_difference(afterSorted.begin(), afterSorted.end(),
                            beforeSorted.begin(), beforeSorted.end(),
                            std::back_inserter(added[perspective]));
    }
    json_array(out, removed[WHITE]);
    out << ',';
    json_array(out, removed[BLACK]);
    out << "],\"added\":[";
    json_array(out, added[WHITE]);
    out << ',';
    json_array(out, added[BLACK]);
    out << "],\"requires_refresh\":[";
    for (Color perspective : {WHITE, BLACK})
    {
        if (perspective != WHITE)
            out << ',';
        out << (dirty != nullptr && ThreatFeatureSet::requires_refresh(*dirty, perspective)
                  ? "true" : "false");
    }
    out << "]}";
}

inline std::string capture_row(const std::string&                 caseId,
                               std::size_t                        sequenceIndex,
                               const std::string&                 operation,
                               const std::string&                 action,
                               const Position&                    position,
                               const NetworkBig&                  network,
                               AccumulatorStack&                  stack,
                               AccumulatorCaches::Cache<1024>&    cache,
                               const ThreatLists&                 before,
                               const DirtyThreats*                dirty) {
    const auto after = active_threats(position);
    const auto state = state_json(position, network, stack, cache);

    auto freshStack = std::make_unique<AccumulatorStack>();
    auto freshCache = std::make_unique<AccumulatorCaches::Cache<1024>>();
    freshStack->reset();
    freshCache->clear(network);
    const auto fresh = state_json(position, network, *freshStack, *freshCache);
    if (state != fresh)
        std::abort();

    std::ostringstream out;
    out << "{\"schema\":\"sf18-big-reference-oracle/v1\""
        << ",\"case\":" << json_string(caseId)
        << ",\"sequence_index\":" << sequenceIndex
        << ",\"operation\":" << json_string(operation)
        << ",\"action\":" << json_string(action)
        << ",\"transition\":";
    if (operation == "root")
        out << "null";
    else
        json_transition(out, before, after, dirty);
    out << ",\"state\":" << state << '}';
    return out.str();
}

inline std::vector<std::string> run(const std::string&              rootFen,
                                    bool                            chess960,
                                    const NetworkBig&               network,
                                    const TranspositionTable&       tt,
                                    const std::string&              caseId,
                                    const std::vector<std::string>& actions) {
    if (caseId.empty())
        std::abort();
    StateListPtr states(new std::deque<StateInfo>(1));
    Position position;
    position.set(rootFen, chess960, &states->back());
    auto stack = std::make_unique<AccumulatorStack>();
    auto cache = std::make_unique<AccumulatorCaches::Cache<1024>>();
    stack->reset();
    cache->clear(network);

    std::vector<std::string> rows;
    rows.reserve(1 + actions.size());
    ThreatLists before;
    rows.push_back(capture_row(caseId, 0, "root", "", position, network,
                               *stack, *cache, before, nullptr));
    std::size_t sequenceIndex = 1;
    for (const auto& action : actions)
    {
        before = active_threats(position);
        if (action == "null")
        {
            if (position.checkers())
                std::abort();
            states->emplace_back();
            position.do_null_move(states->back(), tt);
            rows.push_back(capture_row(caseId, sequenceIndex++, "push_null", action,
                                       position, network, *stack, *cache, before, nullptr));
            continue;
        }

        const Move move = UCIEngine::to_move(position, action);
        if (move == Move::none() || !position.legal(move))
            std::abort();
        states->emplace_back();
        auto [dirtyPiece, dirtyThreats] = stack->push();
        position.do_move(move, states->back(), position.gives_check(move), dirtyPiece,
                         dirtyThreats, &tt, nullptr);
        rows.push_back(capture_row(caseId, sequenceIndex++, "push", action,
                                   position, network, *stack, *cache, before, &dirtyThreats));
    }
    return rows;
}

}  // namespace Stockfish::Eval::NNUE::SF18BigReferenceOracle

#endif
