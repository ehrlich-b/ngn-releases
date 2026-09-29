#include <algorithm>
#include <charconv>
#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

#include <chess.hpp>

int main(int argc, char** argv) {
    if (argc != 4) {
        std::cerr << "usage: convert_uci_openings SOURCE OUTPUT COUNT\n";
        return 2;
    }
    int count = 0;
    const std::string count_text = argv[3];
    const auto parsed = std::from_chars(count_text.data(), count_text.data() + count_text.size(), count);
    if (parsed.ec != std::errc{} || count < 1) throw std::runtime_error("invalid count");
    std::ifstream input(argv[1]);
    std::ofstream output(argv[2], std::ios::out | std::ios::trunc);
    if (!input || !output) throw std::runtime_error("could not open input/output");
    std::string line;
    for (int game = 1; game <= count; ++game) {
        if (!std::getline(input, line) || line.empty()) throw std::runtime_error("source corpus too short");
        std::istringstream tokens(line);
        std::vector<std::string> uci_moves;
        for (std::string move; tokens >> move;) uci_moves.push_back(move);
        if (uci_moves.empty()) throw std::runtime_error("empty opening line");
        chess::Board board(chess::constants::STARTPOS);
        std::vector<std::string> san_moves;
        for (const auto& uci : uci_moves) {
            chess::Movelist legal;
            chess::movegen::legalmoves(legal, board);
            const auto move = chess::uci::uciToMove(board, uci);
            if (std::find(legal.begin(), legal.end(), move) == legal.end()) {
                throw std::runtime_error("illegal opening move in game " + std::to_string(game) + ": " + uci);
            }
            san_moves.push_back(chess::uci::moveToSan(board, move));
            board.makeMove<true>(move);
        }
        output << "[Event \"NGN B0 pinned opening\"]\n"
               << "[Site \"WSL\"]\n"
               << "[Round \"" << game << "\"]\n"
               << "[Result \"*\"]\n\n";
        for (std::size_t ply = 0; ply < san_moves.size(); ++ply) {
            if (ply % 2 == 0) output << (ply / 2 + 1) << ". ";
            output << san_moves[ply] << ' ';
        }
        output << "*\n\n";
    }
    if (!output) throw std::runtime_error("failed writing PGN");
    return 0;
}
